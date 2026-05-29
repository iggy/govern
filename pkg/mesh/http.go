package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// HTTPServer is a thin LOCAL control socket. The govern CLI talks to its own
// node over this API; the node then fans the request out across the mesh via
// libp2p (GossipSub broadcast or targeted RPC streams). It is intended to
// listen on localhost only – it is not the node-to-node transport.
type HTTPServer struct {
	service *Service
	server  *http.Server
	logger  zerolog.Logger
}

func NewHTTPServer(service *Service, addr string, logger zerolog.Logger) *HTTPServer {
	h := &HTTPServer{
		service: service,
		logger:  logger.With().Str("component", "mesh-http").Logger(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", h.handleStatus)
	mux.HandleFunc("/nodes", h.handleNodes)
	mux.HandleFunc("/exec", h.handleLocal)         // run on local node only
	mux.HandleFunc("/broadcast", h.handleBroadcast) // fan out to whole mesh
	mux.HandleFunc("/dispatch", h.handleDispatch)   // targeted to specific peers

	h.server = &http.Server{Addr: addr, Handler: mux}
	return h
}

func (h *HTTPServer) Start() error {
	h.logger.Info().Str("addr", h.server.Addr).Msg("starting local control socket")
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Stop(ctx context.Context) error {
	h.logger.Info().Msg("stopping local control socket")
	return h.server.Shutdown(ctx)
}

func (h *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.service.GetStatus())
}

func (h *HTTPServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.service.GetNodes())
}

// handleLocal runs a command on the local node only.
func (h *HTTPServer) handleLocal(w http.ResponseWriter, r *http.Request) {
	cmd, ctx, cancel, ok := h.decodeCommand(w, r)
	if !ok {
		return
	}
	defer cancel()
	writeJSON(w, h.service.ExecuteCommand(ctx, cmd))
}

// handleBroadcast fans a command out to all matching mesh peers and collects
// results until the timeout elapses.
func (h *HTTPServer) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	cmd, ctx, cancel, ok := h.decodeCommand(w, r)
	if !ok {
		return
	}
	defer cancel()

	results, err := h.service.Broadcast(ctx, cmd)
	if err != nil {
		h.logger.Error().Err(err).Msg("broadcast failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, results)
}

// handleDispatch sends a command directly to the peers named in ?peers=a,b,c
// (or the command's Target.PeerIDs) and collects their replies.
func (h *HTTPServer) handleDispatch(w http.ResponseWriter, r *http.Request) {
	cmd, ctx, cancel, ok := h.decodeCommand(w, r)
	if !ok {
		return
	}
	defer cancel()

	peerIDs := cmd.Target.PeerIDs
	if len(peerIDs) == 0 {
		http.Error(w, "no target peers specified", http.StatusBadRequest)
		return
	}

	results, err := h.service.Dispatch(ctx, cmd, peerIDs)
	if err != nil {
		h.logger.Error().Err(err).Msg("dispatch failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, results)
}

// decodeCommand parses a Command from the request body and derives a timeout
// context from the ?timeout=<seconds> query parameter (default 30s).
func (h *HTTPServer) decodeCommand(w http.ResponseWriter, r *http.Request) (Command, context.Context, context.CancelFunc, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return Command{}, nil, nil, false
	}
	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return Command{}, nil, nil, false
	}
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}

	timeout := 30 * time.Second
	if ts := r.URL.Query().Get("timeout"); ts != "" {
		if t, err := strconv.Atoi(ts); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	return cmd, ctx, cancel, true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Client is the CLI-side helper for talking to a local node's control socket.
type Client struct {
	baseURL string
	client  *http.Client
	logger  zerolog.Logger
}

func NewClient(baseURL string, logger zerolog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
		logger:  logger.With().Str("component", "mesh-client").Logger(),
	}
}

func (c *Client) GetStatus(ctx context.Context) (*MeshStatus, error) {
	var status MeshStatus
	if err := c.getJSON(ctx, "/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]Node, error) {
	var nodes []Node
	if err := c.getJSON(ctx, "/nodes", &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// ExecuteLocal runs a command on the connected node only.
func (c *Client) ExecuteLocal(ctx context.Context, cmd Command, timeout time.Duration) (*CommandResult, error) {
	var result CommandResult
	if err := c.postJSON(ctx, "/exec", cmd, timeout, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Broadcast fans a command out to the whole mesh and returns all collected results.
func (c *Client) Broadcast(ctx context.Context, cmd Command, timeout time.Duration) ([]*CommandResult, error) {
	var results []*CommandResult
	if err := c.postJSON(ctx, "/broadcast", cmd, timeout, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// Dispatch sends a command to the peers listed in cmd.Target.PeerIDs.
func (c *Client) Dispatch(ctx context.Context, cmd Command, timeout time.Duration) ([]*CommandResult, error) {
	var results []*CommandResult
	if err := c.postJSON(ctx, "/dispatch", cmd, timeout, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) postJSON(ctx context.Context, path string, body interface{}, timeout time.Duration, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s?timeout=%d", c.baseURL, path, int(timeout.Seconds()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
