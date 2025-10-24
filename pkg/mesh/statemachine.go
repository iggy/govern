package mesh

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/lni/dragonboat/v4/statemachine"
)

type MeshStateMachine struct {
	ShardID   uint64
	ReplicaID uint64
	commands  map[string]*Command
	results   map[string]*CommandResult
}

func NewMeshStateMachine(shardID, replicaID uint64) statemachine.IStateMachine {
	return &MeshStateMachine{
		ShardID:   shardID,
		ReplicaID: replicaID,
		commands:  make(map[string]*Command),
		results:   make(map[string]*CommandResult),
	}
}

func (m *MeshStateMachine) Update(entry statemachine.Entry) (statemachine.Result, error) {
	var cmd Command
	if err := json.Unmarshal(entry.Cmd, &cmd); err != nil {
		return statemachine.Result{}, err
	}

	m.commands[cmd.ID] = &cmd

	return statemachine.Result{Value: uint64(len(entry.Cmd))}, nil
}

func (m *MeshStateMachine) Lookup(query interface{}) (interface{}, error) {
	data, ok := query.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	if len(data) == 0 {
		return m.getAllCommands()
	}

	var queryData map[string]interface{}
	if err := json.Unmarshal(data, &queryData); err != nil {
		return nil, err
	}

	if cmdID, exists := queryData["command_id"]; exists {
		if id, ok := cmdID.(string); ok {
			if cmd, found := m.commands[id]; found {
				return json.Marshal(cmd)
			}
		}
	}

	if resultID, exists := queryData["result_id"]; exists {
		if id, ok := resultID.(string); ok {
			if result, found := m.results[id]; found {
				return json.Marshal(result)
			}
		}
	}

	return m.getAllCommands()
}

func (m *MeshStateMachine) getAllCommands() ([]byte, error) {
	commands := make([]*Command, 0, len(m.commands))
	for _, cmd := range m.commands {
		commands = append(commands, cmd)
	}
	return json.Marshal(commands)
}

func (m *MeshStateMachine) SaveSnapshot(w io.Writer, fc statemachine.ISnapshotFileCollection, done <-chan struct{}) error {
	data := struct {
		Commands map[string]*Command       `json:"commands"`
		Results  map[string]*CommandResult `json:"results"`
	}{
		Commands: m.commands,
		Results:  m.results,
	}

	return json.NewEncoder(w).Encode(data)
}

func (m *MeshStateMachine) RecoverFromSnapshot(r io.Reader, files []statemachine.SnapshotFile, done <-chan struct{}) error {
	var data struct {
		Commands map[string]*Command       `json:"commands"`
		Results  map[string]*CommandResult `json:"results"`
	}

	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return err
	}

	m.commands = data.Commands
	if m.commands == nil {
		m.commands = make(map[string]*Command)
	}

	m.results = data.Results
	if m.results == nil {
		m.results = make(map[string]*CommandResult)
	}

	return nil
}

func (m *MeshStateMachine) Close() error {
	return nil
}

func (m *MeshStateMachine) StoreResult(result *CommandResult) {
	m.results[result.ID] = result
}
