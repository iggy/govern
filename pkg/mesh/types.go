package mesh

import (
	"context"
	"encoding/json"
	"time"
)

type CommandType string

const (
	CommandTypeExec      CommandType = "exec"
	CommandTypeFacts     CommandType = "facts"
	CommandTypeApplyLaws CommandType = "apply_laws"
)

// Command is the unit of work dispatched across the mesh. It is transport
// agnostic: the same struct is published to the GossipSub broadcast topic for
// fan-out and sent over a direct stream for targeted request/reply.
type Command struct {
	ID        string            `json:"id"`
	Type      CommandType       `json:"type"`
	Payload   json.RawMessage   `json:"payload"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Target    Target            `json:"target,omitempty"`
	Origin    string            `json:"origin,omitempty"` // PeerID of the initiator
	Timestamp time.Time         `json:"timestamp"`
}

// Target selects which nodes should act on a broadcast Command. An empty Target
// matches every node. Selectors are evaluated locally by each receiving node
// against its own facts, so there is no central place that needs to know the
// fleet inventory.
type Target struct {
	// Hostnames, if set, restricts execution to nodes whose hostname is listed.
	Hostnames []string `json:"hostnames,omitempty"`
	// PeerIDs, if set, restricts execution to the listed peer identities.
	PeerIDs []string `json:"peer_ids,omitempty"`
	// All forces a match on every node regardless of other fields.
	All bool `json:"all,omitempty"`
}

// IsEmpty reports whether the target matches all nodes by default.
func (t Target) IsEmpty() bool {
	return t.All || (len(t.Hostnames) == 0 && len(t.PeerIDs) == 0)
}

type ExecPayload struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
}

type FactsPayload struct {
	Categories []string `json:"categories,omitempty"`
}

type ApplyLawsPayload struct {
	LawFiles []string `json:"law_files"`
	DryRun   bool     `json:"dry_run,omitempty"`
}

// CommandResult is what a node returns after acting on a Command. Results flow
// back over the GossipSub results topic (for broadcasts) or directly down the
// request stream (for targeted RPC).
type CommandResult struct {
	ID        string          `json:"id"`        // matches Command.ID
	PeerID    string          `json:"peer_id"`   // which node produced this result
	Hostname  string          `json:"hostname"`  // human-friendly node identity
	Success   bool            `json:"success"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Node is a discovered mesh peer, reported by status/nodes queries.
type Node struct {
	PeerID   string   `json:"peer_id"`
	Addrs    []string `json:"addrs,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
}

// MeshStatus summarizes the local node's view of the mesh. There is no leader
// and no shared cluster state – this is simply what this node currently knows.
type MeshStatus struct {
	PeerID     string    `json:"peer_id"`
	Hostname   string    `json:"hostname"`
	Addrs      []string  `json:"addrs"`
	Rendezvous string    `json:"rendezvous"`
	Peers      []Node    `json:"peers"`
	Timestamp  time.Time `json:"timestamp"`
}

// MeshService is the transport-agnostic contract the CLI/HTTP layers depend on.
type MeshService interface {
	Start(ctx context.Context) error
	Stop() error
	// ExecuteCommand runs a command on the local node only.
	ExecuteCommand(ctx context.Context, cmd Command) *CommandResult
	// Broadcast fans a command out to all matching peers and collects results
	// until the context deadline elapses.
	Broadcast(ctx context.Context, cmd Command) ([]*CommandResult, error)
	// Dispatch sends a command directly to specific peers and collects their
	// replies (targeted request/reply).
	Dispatch(ctx context.Context, cmd Command, peerIDs []string) ([]*CommandResult, error)
	GetStatus() *MeshStatus
	GetNodes() []Node
}
