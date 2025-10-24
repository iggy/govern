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
	mux.HandleFunc("/command", h.handleCommand)
	mux.HandleFunc("/broadcast", h.handleBroadcast)

	h.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return h
}

func (h *HTTPServer) Start() error {
	h.logger.Info().Str("addr", h.server.Addr).Msg("starting HTTP server")
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Stop(ctx context.Context) error {
	h.logger.Info().Msg("stopping HTTP server")
	return h.server.Shutdown(ctx)
}

func (h *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, err := h.service.GetStatus()
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get status")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *HTTPServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes, err := h.service.GetNodes()
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get nodes")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (h *HTTPServer) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}

	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 30 * time.Second
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result, err := h.service.ExecuteCommand(ctx, cmd)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to execute command")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *HTTPServer) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}

	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 30 * time.Second
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	results, err := h.service.BroadcastCommand(ctx, cmd)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to broadcast command")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type Client struct {
	baseURL string
	client  *http.Client
	logger  zerolog.Logger
}

func NewClient(baseURL string, logger zerolog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  logger.With().Str("component", "mesh-client").Logger(),
	}
}

func (c *Client) GetStatus(ctx context.Context) (*MeshStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/status", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var status MeshStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/nodes", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var nodes []Node
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}

func (c *Client) ExecuteCommand(ctx context.Context, cmd Command) (*CommandResult, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/command",
		bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result CommandResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) BroadcastCommand(ctx context.Context, cmd Command) (map[uint64]*CommandResult, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/broadcast",
		bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var results map[uint64]*CommandResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}
