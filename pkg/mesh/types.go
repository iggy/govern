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

type Command struct {
	ID        string            `json:"id"`
	Type      CommandType       `json:"type"`
	Payload   json.RawMessage   `json:"payload"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
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

type CommandResult struct {
	ID        string          `json:"id"`
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

type Node struct {
	ID      uint64 `json:"id"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

type MeshStatus struct {
	NodeID    uint64    `json:"node_id"`
	ShardID   uint64    `json:"shard_id"`
	IsLeader  bool      `json:"is_leader"`
	Nodes     []Node    `json:"nodes"`
	Timestamp time.Time `json:"timestamp"`
}

type MeshService interface {
	Start(ctx context.Context) error
	Stop() error
	ExecuteCommand(ctx context.Context, cmd Command) (*CommandResult, error)
	BroadcastCommand(ctx context.Context, cmd Command) (map[uint64]*CommandResult, error)
	GetStatus() (*MeshStatus, error)
	GetNodes() ([]Node, error)
}
