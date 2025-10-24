package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/lni/goutils/syncutil"
	"github.com/rs/zerolog"

	"github.com/iggy/govern/pkg/facts"
	"github.com/iggy/govern/pkg/laws"
)

const (
	DefaultShardID = 1
)

type Service struct {
	nodeHost  *dragonboat.NodeHost
	shardID   uint64
	replicaID uint64
	dataDir   string
	raftAddr  string
	stopper   *syncutil.Stopper
	logger    zerolog.Logger
}

type Config struct {
	ReplicaID          uint64
	ShardID            uint64
	RaftAddress        string
	DataDir            string
	InitialMembers     map[uint64]string
	Join               bool
	RTTMillisecond     uint64
	ElectionRTT        uint64
	HeartbeatRTT       uint64
	SnapshotEntries    uint64
	CompactionOverhead uint64
}

func NewService(cfg Config, logger zerolog.Logger) (*Service, error) {
	if cfg.ShardID == 0 {
		cfg.ShardID = DefaultShardID
	}
	if cfg.RTTMillisecond == 0 {
		cfg.RTTMillisecond = 200
	}
	if cfg.ElectionRTT == 0 {
		cfg.ElectionRTT = 10
	}
	if cfg.HeartbeatRTT == 0 {
		cfg.HeartbeatRTT = 1
	}
	if cfg.SnapshotEntries == 0 {
		cfg.SnapshotEntries = 10
	}
	if cfg.CompactionOverhead == 0 {
		cfg.CompactionOverhead = 5
	}

	dataDir := filepath.Join(cfg.DataDir, fmt.Sprintf("node%d", cfg.ReplicaID))

	nhc := config.NodeHostConfig{
		WALDir:         dataDir,
		NodeHostDir:    dataDir,
		RTTMillisecond: cfg.RTTMillisecond,
		RaftAddress:    cfg.RaftAddress,
	}

	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeHost: %w", err)
	}

	rc := config.Config{
		ReplicaID:          cfg.ReplicaID,
		ShardID:            cfg.ShardID,
		ElectionRTT:        cfg.ElectionRTT,
		HeartbeatRTT:       cfg.HeartbeatRTT,
		CheckQuorum:        true,
		SnapshotEntries:    cfg.SnapshotEntries,
		CompactionOverhead: cfg.CompactionOverhead,
	}

	createSM := func(shardID, replicaID uint64) statemachine.IStateMachine {
		return NewMeshStateMachine(shardID, replicaID)
	}

	if err := nh.StartReplica(cfg.InitialMembers, cfg.Join, createSM, rc); err != nil {
		nh.Close()
		return nil, fmt.Errorf("failed to start replica: %w", err)
	}

	return &Service{
		nodeHost:  nh,
		shardID:   cfg.ShardID,
		replicaID: cfg.ReplicaID,
		dataDir:   dataDir,
		raftAddr:  cfg.RaftAddress,
		stopper:   syncutil.NewStopper(),
		logger:    logger.With().Str("component", "mesh").Logger(),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.logger.Info().Msg("starting mesh service")

	s.stopper.RunWorker(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				status, err := s.GetStatus()
				if err != nil {
					s.logger.Error().Err(err).Msg("failed to get mesh status")
					continue
				}
				s.logger.Debug().
					Bool("is_leader", status.IsLeader).
					Int("node_count", len(status.Nodes)).
					Msg("mesh status")

			case <-s.stopper.ShouldStop():
				return
			case <-ctx.Done():
				return
			}
		}
	})

	return nil
}

func (s *Service) Stop() error {
	s.logger.Info().Msg("stopping mesh service")
	s.stopper.Stop()
	s.nodeHost.Close()
	return nil
}

func (s *Service) ExecuteCommand(ctx context.Context, cmd Command) (*CommandResult, error) {
	result := &CommandResult{
		ID:        cmd.ID,
		Timestamp: time.Now(),
	}

	switch cmd.Type {
	case CommandTypeExec:
		execResult, err := s.executeExecCommand(ctx, cmd)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.Output, _ = json.Marshal(execResult)
		}

	case CommandTypeFacts:
		factsResult, err := s.executeFactsCommand(ctx, cmd)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.Output, _ = json.Marshal(factsResult)
		}

	case CommandTypeApplyLaws:
		lawsResult, err := s.executeApplyLawsCommand(ctx, cmd)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.Output, _ = json.Marshal(lawsResult)
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown command type: %s", cmd.Type)
	}

	return result, nil
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
	if len(payload.Env) > 0 {
		for k, v := range payload.Env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdout, err := execCmd.Output()
	result := &ExecResult{
		Stdout: string(stdout),
	}

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
			results[lawFile] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			continue
		}

		if payload.DryRun {
			results[lawFile] = map[string]interface{}{
				"success":   true,
				"dry_run":   true,
				"law_count": len(vertices),
				"message":   "would apply laws (dry run)",
			}
		} else {
			applied := 0
			errors := []string{}
			for _, vertex := range vertices {
				lawNode := vertex.Label()
				if lawNode.Law != nil {
					if err := lawNode.Law.Ensure(false); err != nil {
						errors = append(errors, fmt.Sprintf("%s: %v", lawNode.Name, err))
					} else {
						applied++
					}
				}
			}

			results[lawFile] = map[string]interface{}{
				"success":    len(errors) == 0,
				"applied":    applied,
				"total_laws": len(vertices),
				"errors":     errors,
			}
		}
	}

	return results, nil
}

func (s *Service) BroadcastCommand(ctx context.Context, cmd Command) (map[uint64]*CommandResult, error) {
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}
	cmd.Timestamp = time.Now()

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	cs := s.nodeHost.GetNoOPSession(s.shardID)
	_, err = s.nodeHost.SyncPropose(ctx, cs, data)
	if err != nil {
		return nil, fmt.Errorf("failed to propose command: %w", err)
	}

	localResult, err := s.ExecuteCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command locally: %w", err)
	}

	results := make(map[uint64]*CommandResult)
	results[s.replicaID] = localResult

	return results, nil
}

func (s *Service) GetStatus() (*MeshStatus, error) {
	leaderID, term, valid, err := s.nodeHost.GetLeaderID(s.shardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get leader ID: %w", err)
	}
	_ = term

	membership, err := s.nodeHost.SyncGetShardMembership(context.Background(), s.shardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shard membership: %w", err)
	}

	nodes := make([]Node, 0, len(membership.Nodes))
	for replicaID, addr := range membership.Nodes {
		status := "follower"
		if valid && replicaID == leaderID {
			status = "leader"
		}
		nodes = append(nodes, Node{
			ID:      replicaID,
			Address: addr,
			Status:  status,
		})
	}

	return &MeshStatus{
		NodeID:    s.replicaID,
		ShardID:   s.shardID,
		IsLeader:  valid && s.replicaID == leaderID,
		Nodes:     nodes,
		Timestamp: time.Now(),
	}, nil
}

func (s *Service) GetNodes() ([]Node, error) {
	status, err := s.GetStatus()
	if err != nil {
		return nil, err
	}
	return status.Nodes, nil
}
