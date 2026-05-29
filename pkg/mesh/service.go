package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/iggy/govern/pkg/facts"
	"github.com/iggy/govern/pkg/laws"
)

const (
	// topicCommands carries Commands fanned out to the whole mesh.
	topicCommands = "govern/commands/v1"
	// topicResults carries CommandResults flowing back to broadcast origins.
	topicResults = "govern/results/v1"
	// rpcProtocol is the direct request/reply stream protocol for targeted
	// commands and fact queries.
	rpcProtocol protocol.ID = "/govern/rpc/1.0.0"
)

// Config configures the mesh Service. It is intentionally small – there are no
// replica IDs, quorum sizes, or membership bookkeeping. A node needs to know
// where to listen, who to bootstrap from, and which mesh (rendezvous) to join.
type Config struct {
	ListenAddrs    []string
	BootstrapPeers []string
	Rendezvous     string
	DataDir        string
	// Private hints the node is behind NAT (enables AutoRelay reservations).
	Private bool
}

// Service is the masterless mesh node. It owns the libp2p host, a GossipSub
// router for broadcast/collect, and a stream handler for targeted RPC.
//
// It replaces the previous Raft (dragonboat) implementation. There is no
// leader, no replicated log, and no shared cluster state: every operation is
// fire-and-forget fan-out with best-effort result collection.
type Service struct {
	host   *Host
	ps     *pubsub.PubSub
	cmdT   *pubsub.Topic
	resT   *pubsub.Topic
	cmdSub *pubsub.Subscription
	resSub *pubsub.Subscription
	logger zerolog.Logger

	hostname string

	// pending maps a broadcast command ID to a channel collecting results from
	// peers. Only the initiator of a broadcast registers a collector.
	mu      sync.Mutex
	pending map[string]chan *CommandResult

	stopOnce sync.Once
	cancel   context.CancelFunc
}

// NewService builds the libp2p host and wires up pubsub + the RPC handler.
func NewService(ctx context.Context, cfg Config, logger zerolog.Logger) (*Service, error) {
	logger = logger.With().Str("component", "mesh").Logger()

	h, err := NewHost(ctx, HostConfig{
		ListenAddrs:              cfg.ListenAddrs,
		BootstrapPeers:           cfg.BootstrapPeers,
		Rendezvous:               cfg.Rendezvous,
		DataDir:                  cfg.DataDir,
		ForceReachabilityPrivate: cfg.Private,
	}, logger)
	if err != nil {
		return nil, err
	}

	ps, err := pubsub.NewGossipSub(ctx, h.Host())
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	cmdTopic, err := ps.Join(topicCommands)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to join commands topic: %w", err)
	}
	resTopic, err := ps.Join(topicResults)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to join results topic: %w", err)
	}

	cmdSub, err := cmdTopic.Subscribe()
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to subscribe to commands: %w", err)
	}
	resSub, err := resTopic.Subscribe()
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to subscribe to results: %w", err)
	}

	hostname, _ := os.Hostname()

	s := &Service{
		host:     h,
		ps:       ps,
		cmdT:     cmdTopic,
		resT:     resTopic,
		cmdSub:   cmdSub,
		resSub:   resSub,
		logger:   logger,
		hostname: hostname,
		pending:  make(map[string]chan *CommandResult),
	}

	// Targeted request/reply: peers open a stream, send one Command, read one
	// CommandResult back. This is the clean analog of Salt's return model.
	h.Host().SetStreamHandler(rpcProtocol, s.handleRPC)

	return s, nil
}

// Start begins advertising on the rendezvous key and consuming the pubsub
// topics. It returns immediately; the loops run until Stop or ctx cancellation.
func (s *Service) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	s.host.Advertise(ctx)

	go s.consumeCommands(ctx)
	go s.consumeResults(ctx)

	s.logger.Info().Str("peer_id", s.host.ID().String()).Msg("mesh service started")
	return nil
}

// Stop shuts down the service and underlying host.
func (s *Service) Stop() error {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.cmdSub.Cancel()
		s.resSub.Cancel()
		_ = s.cmdT.Close()
		_ = s.resT.Close()
	})
	return s.host.Close()
}

// consumeCommands processes broadcast Commands targeted at this node.
func (s *Service) consumeCommands(ctx context.Context) {
	self := s.host.ID()
	for {
		msg, err := s.cmdSub.Next(ctx)
		if err != nil {
			return // context cancelled
		}
		// Ignore our own broadcasts; the initiator runs locally itself.
		if msg.ReceivedFrom == self || (msg.GetFrom() == self) {
			continue
		}

		var cmd Command
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			s.logger.Warn().Err(err).Msg("dropping malformed command")
			continue
		}

		if !s.matchesTarget(cmd.Target) {
			continue
		}

		go s.runAndReturn(ctx, cmd)
	}
}

// runAndReturn executes a broadcast command locally and publishes the result
// back on the results topic for the origin to collect.
func (s *Service) runAndReturn(ctx context.Context, cmd Command) {
	result := s.ExecuteCommand(ctx, cmd)
	data, err := json.Marshal(result)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal result")
		return
	}
	if err := s.resT.Publish(ctx, data); err != nil {
		s.logger.Error().Err(err).Msg("failed to publish result")
	}
}

// consumeResults delivers incoming results to the collector waiting on the
// matching command ID (if this node initiated that broadcast).
func (s *Service) consumeResults(ctx context.Context) {
	for {
		msg, err := s.resSub.Next(ctx)
		if err != nil {
			return
		}
		var result CommandResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			continue
		}
		s.mu.Lock()
		ch, ok := s.pending[result.ID]
		s.mu.Unlock()
		if ok {
			select {
			case ch <- &result:
			case <-ctx.Done():
				return
			default:
				// collector buffer full; drop to avoid blocking the consumer
			}
		}
	}
}

// Broadcast publishes a command to the whole mesh and collects results from all
// matching peers until the context deadline. The local node is included.
func (s *Service) Broadcast(ctx context.Context, cmd Command) ([]*CommandResult, error) {
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}
	cmd.Origin = s.host.ID().String()
	cmd.Timestamp = time.Now()

	collector := make(chan *CommandResult, 256)
	s.mu.Lock()
	s.pending[cmd.ID] = collector
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, cmd.ID)
		s.mu.Unlock()
	}()

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	if err := s.cmdT.Publish(ctx, data); err != nil {
		return nil, fmt.Errorf("failed to publish command: %w", err)
	}

	results := []*CommandResult{}
	// Always include the local result if this node matches the target.
	if s.matchesTarget(cmd.Target) {
		results = append(results, s.ExecuteCommand(ctx, cmd))
	}

	for {
		select {
		case r := <-collector:
			results = append(results, r)
		case <-ctx.Done():
			return results, nil
		}
	}
}

// Dispatch sends a command directly to specific peers over an RPC stream and
// collects their replies. This is targeted request/reply – no pubsub involved.
func (s *Service) Dispatch(ctx context.Context, cmd Command, peerIDs []string) ([]*CommandResult, error) {
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}
	cmd.Origin = s.host.ID().String()
	cmd.Timestamp = time.Now()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []*CommandResult
	)
	for _, pidStr := range peerIDs {
		pid, err := peer.Decode(pidStr)
		if err != nil {
			mu.Lock()
			results = append(results, &CommandResult{
				ID: cmd.ID, PeerID: pidStr, Success: false,
				Error: fmt.Sprintf("invalid peer id: %v", err), Timestamp: time.Now(),
			})
			mu.Unlock()
			continue
		}

		// A targeted command to ourselves runs locally.
		if pid == s.host.ID() {
			r := s.ExecuteCommand(ctx, cmd)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()
			r := s.sendRPC(ctx, pid, cmd)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(pid)
	}
	wg.Wait()
	return results, nil
}

// sendRPC opens a stream to a peer, sends the command, and reads one result.
func (s *Service) sendRPC(ctx context.Context, pid peer.ID, cmd Command) *CommandResult {
	fail := func(err error) *CommandResult {
		return &CommandResult{
			ID: cmd.ID, PeerID: pid.String(), Success: false,
			Error: err.Error(), Timestamp: time.Now(),
		}
	}

	// Ensure we have addresses for the peer; resolve via DHT if not.
	if len(s.host.Host().Peerstore().Addrs(pid)) == 0 {
		if info, err := s.host.FindPeer(ctx, pid); err == nil {
			s.host.Host().Peerstore().AddAddrs(pid, info.Addrs, time.Hour)
		}
	}

	stream, err := s.host.Host().NewStream(ctx, pid, rpcProtocol)
	if err != nil {
		return fail(fmt.Errorf("failed to open stream: %w", err))
	}
	defer stream.Close()

	if dl, ok := ctx.Deadline(); ok {
		_ = stream.SetDeadline(dl)
	}

	if err := json.NewEncoder(stream).Encode(cmd); err != nil {
		return fail(fmt.Errorf("failed to send command: %w", err))
	}
	_ = stream.CloseWrite()

	var result CommandResult
	if err := json.NewDecoder(stream).Decode(&result); err != nil {
		return fail(fmt.Errorf("failed to read result: %w", err))
	}
	return &result
}

// handleRPC serves an inbound targeted command: read one Command, execute it,
// write one CommandResult.
func (s *Service) handleRPC(stream network.Stream) {
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Minute))

	var cmd Command
	if err := json.NewDecoder(stream).Decode(&cmd); err != nil {
		s.logger.Warn().Err(err).Msg("failed to decode RPC command")
		return
	}

	result := s.ExecuteCommand(context.Background(), cmd)
	if err := json.NewEncoder(stream).Encode(result); err != nil {
		s.logger.Warn().Err(err).Msg("failed to encode RPC result")
	}
}

// matchesTarget evaluates a Command's Target against this node's identity.
// Selectors are checked locally, so no node needs a global inventory.
func (s *Service) matchesTarget(t Target) bool {
	if t.IsEmpty() {
		return true
	}
	for _, h := range t.Hostnames {
		if h == s.hostname {
			return true
		}
	}
	self := s.host.ID().String()
	for _, p := range t.PeerIDs {
		if p == self {
			return true
		}
	}
	return false
}

// ExecuteCommand runs a command on the local node and returns its result.
func (s *Service) ExecuteCommand(ctx context.Context, cmd Command) *CommandResult {
	result := &CommandResult{
		ID:        cmd.ID,
		PeerID:    s.host.ID().String(),
		Hostname:  s.hostname,
		Timestamp: time.Now(),
	}

	var (
		out interface{}
		err error
	)
	switch cmd.Type {
	case CommandTypeExec:
		out, err = s.executeExecCommand(ctx, cmd)
	case CommandTypeFacts:
		out, err = s.executeFactsCommand(ctx, cmd)
	case CommandTypeApplyLaws:
		out, err = s.executeApplyLawsCommand(ctx, cmd)
	default:
		err = fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
		result.Output, _ = json.Marshal(out)
	}
	return result
}

func (s *Service) executeExecCommand(ctx context.Context, cmd Command) (*ExecResult, error) {
	var payload ExecPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid exec payload: %w", err)
	}

	execCmd := exec.CommandContext(ctx, payload.Command, payload.Args...)
	if payload.WorkDir != "" {
		execCmd.Dir = payload.WorkDir
	}
	for k, v := range payload.Env {
		execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := execCmd.Output()
	result := &ExecResult{Stdout: string(stdout)}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
		} else {
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	}
	return result, nil
}

func (s *Service) executeFactsCommand(ctx context.Context, cmd Command) (interface{}, error) {
	var payload FactsPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid facts payload: %w", err)
	}
	return facts.Facts, nil
}

func (s *Service) executeApplyLawsCommand(ctx context.Context, cmd Command) (interface{}, error) {
	var payload ApplyLawsPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid apply laws payload: %w", err)
	}

	results := make(map[string]interface{})
	for _, lawFile := range payload.LawFiles {
		vertices, err := laws.ParseFiles(lawFile)
		if err != nil {
			results[lawFile] = map[string]interface{}{"success": false, "error": err.Error()}
			continue
		}

		if payload.DryRun {
			results[lawFile] = map[string]interface{}{
				"success":   true,
				"dry_run":   true,
				"law_count": len(vertices),
				"message":   "would apply laws (dry run)",
			}
			continue
		}

		applied := 0
		errs := []string{}
		for _, vertex := range vertices {
			lawNode := vertex.Label()
			if lawNode.Law != nil {
				if err := lawNode.Law.Ensure(false); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", lawNode.Name, err))
				} else {
					applied++
				}
			}
		}
		results[lawFile] = map[string]interface{}{
			"success":    len(errs) == 0,
			"applied":    applied,
			"total_laws": len(vertices),
			"errors":     errs,
		}
	}
	return results, nil
}

// GetStatus returns this node's current view of the mesh.
func (s *Service) GetStatus() *MeshStatus {
	return &MeshStatus{
		PeerID:     s.host.ID().String(),
		Hostname:   s.hostname,
		Addrs:      multiaddrsToStrings(s.host.Addrs()),
		Rendezvous: s.host.rendezvous,
		Peers:      s.GetNodes(),
		Timestamp:  time.Now(),
	}
}

// GetNodes returns the peers this node currently knows about.
func (s *Service) GetNodes() []Node {
	peers := s.host.Peers()
	nodes := make([]Node, 0, len(peers))
	for _, p := range peers {
		addrs := make([]string, 0, len(p.Addrs))
		for _, a := range p.Addrs {
			addrs = append(addrs, a.String())
		}
		nodes = append(nodes, Node{PeerID: p.ID.String(), Addrs: addrs})
	}
	return nodes
}
