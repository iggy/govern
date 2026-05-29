package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/iggy/govern/pkg/mesh"
)

// Shared targeting flags. A govern CLI invocation always connects to the LOCAL
// node's control socket; the local node then fans the request out across the
// mesh according to the selected target.
var (
	meshControl string // local control socket address

	meshTargetAll   bool     // broadcast to every node
	meshTargetHosts []string // broadcast, but only these hostnames act
	meshTargetPeers []string // targeted RPC to these specific peer IDs
	meshTimeout     int      // seconds to wait for results
)

// addTargetFlags wires the common targeting flags onto a command.
func addTargetFlags(c *cobra.Command) {
	c.Flags().StringVar(&meshControl, "control", "127.0.0.1:8008", "Local node control socket (host:port)")
	c.Flags().BoolVar(&meshTargetAll, "all", false, "Apply to all nodes in the mesh (broadcast)")
	c.Flags().StringSliceVar(&meshTargetHosts, "target", nil, "Broadcast but only act on these hostnames")
	c.Flags().StringSliceVar(&meshTargetPeers, "peer", nil, "Send directly to these peer IDs (targeted RPC)")
	c.Flags().IntVar(&meshTimeout, "timeout", 30, "Seconds to wait for results")
}

func meshClient() *mesh.Client {
	return mesh.NewClient(fmt.Sprintf("http://%s", meshControl), log.Logger)
}

func printJSON(v interface{}) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to marshal output")
	}
	fmt.Println(string(out))
}

// runCommand dispatches a built Command according to the targeting flags:
//   - --peer  → targeted RPC (Dispatch) to specific peers
//   - --all / --target → broadcast and collect
//   - neither → run on the local node only
func runCommand(cmdType mesh.CommandType, payload interface{}) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to marshal payload")
	}

	command := mesh.Command{Type: cmdType, Payload: payloadData}

	timeout := time.Duration(meshTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout+10*time.Second)
	defer cancel()

	client := meshClient()

	switch {
	case len(meshTargetPeers) > 0:
		command.Target.PeerIDs = meshTargetPeers
		results, err := client.Dispatch(ctx, command, timeout)
		if err != nil {
			log.Fatal().Err(err).Msg("dispatch failed")
		}
		printJSON(results)

	case meshTargetAll || len(meshTargetHosts) > 0:
		command.Target = mesh.Target{All: meshTargetAll, Hostnames: meshTargetHosts}
		results, err := client.Broadcast(ctx, command, timeout)
		if err != nil {
			log.Fatal().Err(err).Msg("broadcast failed")
		}
		printJSON(results)

	default:
		result, err := client.ExecuteLocal(ctx, command, timeout)
		if err != nil {
			log.Fatal().Err(err).Msg("local execution failed")
		}
		printJSON(result)
	}
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get mesh status",
	Long:  `Show the local node's view of the mesh: its identity, addresses, and known peers. There is no leader – this is simply what this node currently knows.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status, err := meshClient().GetStatus(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get mesh status")
		}
		printJSON(status)
	},
}

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List known mesh peers",
	Long:  `List the peers the local node has discovered in the mesh.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		nodes, err := meshClient().GetNodes(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get mesh nodes")
		}
		printJSON(nodes)
	},
}

var (
	meshExecCommand string
	meshExecArgs    []string
	meshExecWorkDir string
	meshExecEnv     []string
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a command across the mesh",
	Long: `Execute a shell command. By default it runs on the local node only.
Use --all to run on every node, --target to restrict to hostnames, or --peer to
send directly to specific peers.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshExecCommand == "" {
			log.Fatal().Msg("command is required (use --command)")
		}
		env := make(map[string]string)
		for _, e := range meshExecEnv {
			if k, v, ok := strings.Cut(e, "="); ok {
				env[k] = v
			}
		}
		runCommand(mesh.CommandTypeExec, mesh.ExecPayload{
			Command: meshExecCommand,
			Args:    meshExecArgs,
			Env:     env,
			WorkDir: meshExecWorkDir,
		})
	},
}

var meshFactsCategories []string

var meshFactsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Query facts across the mesh",
	Long: `Query system facts. By default it queries the local node only. Use --all,
--target, or --peer to query other nodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		runCommand(mesh.CommandTypeFacts, mesh.FactsPayload{Categories: meshFactsCategories})
	},
}

var (
	meshApplyFiles  []string
	meshApplyDryRun bool
)

var meshApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply laws across the mesh",
	Long: `Apply governance laws. By default it applies on the local node only. Use
--all, --target, or --peer to apply on other nodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(meshApplyFiles) == 0 {
			log.Fatal().Msg("at least one law file is required (use --files)")
		}
		runCommand(mesh.CommandTypeApplyLaws, mesh.ApplyLawsPayload{
			LawFiles: meshApplyFiles,
			DryRun:   meshApplyDryRun,
		})
	},
}

func init() {
	meshCmd.AddCommand(statusCmd)
	meshCmd.AddCommand(nodesCmd)
	meshCmd.AddCommand(execCmd)
	meshCmd.AddCommand(meshFactsCmd)
	meshCmd.AddCommand(meshApplyCmd)

	// status / nodes only need to reach the local control socket.
	statusCmd.Flags().StringVar(&meshControl, "control", "127.0.0.1:8008", "Local node control socket (host:port)")
	nodesCmd.Flags().StringVar(&meshControl, "control", "127.0.0.1:8008", "Local node control socket (host:port)")

	// exec
	addTargetFlags(execCmd)
	execCmd.Flags().StringVar(&meshExecCommand, "command", "", "Command to execute")
	execCmd.Flags().StringSliceVar(&meshExecArgs, "args", nil, "Command arguments")
	execCmd.Flags().StringVar(&meshExecWorkDir, "workdir", "", "Working directory")
	execCmd.Flags().StringSliceVar(&meshExecEnv, "env", nil, "Environment variables (KEY=VALUE)")
	execCmd.MarkFlagRequired("command")

	// facts
	addTargetFlags(meshFactsCmd)
	meshFactsCmd.Flags().StringSliceVar(&meshFactsCategories, "categories", nil, "Fact categories to retrieve")

	// apply
	addTargetFlags(meshApplyCmd)
	meshApplyCmd.Flags().StringSliceVar(&meshApplyFiles, "files", nil, "Law files to apply")
	meshApplyCmd.Flags().BoolVar(&meshApplyDryRun, "dry-run", false, "Perform dry run without applying changes")
	meshApplyCmd.MarkFlagRequired("files")
}
