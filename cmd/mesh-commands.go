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

var (
	meshStatusNode string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get mesh status",
	Long:  `Get the status of the mesh cluster including node information and leadership.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshStatusNode == "" {
			log.Fatal().Msg("node address is required (use --node)")
		}

		client := mesh.NewClient(fmt.Sprintf("http://%s", meshStatusNode), log.Logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get mesh status")
		}

		output, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal status")
		}

		fmt.Println(string(output))
	},
}

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List mesh nodes",
	Long:  `List all nodes in the mesh cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshStatusNode == "" {
			log.Fatal().Msg("node address is required (use --node)")
		}

		client := mesh.NewClient(fmt.Sprintf("http://%s", meshStatusNode), log.Logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := client.GetNodes(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get mesh nodes")
		}

		output, err := json.MarshalIndent(nodes, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal nodes")
		}

		fmt.Println(string(output))
	},
}

var (
	meshExecNode    string
	meshExecCommand string
	meshExecArgs    []string
	meshExecWorkDir string
	meshExecEnv     []string
	meshExecTimeout int
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute command on mesh node",
	Long:  `Execute a command on a specific mesh node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshExecNode == "" {
			log.Fatal().Msg("node address is required (use --node)")
		}
		if meshExecCommand == "" {
			log.Fatal().Msg("command is required (use --command)")
		}

		env := make(map[string]string)
		for _, e := range meshExecEnv {
			parts := strings.Split(e, "=")
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}

		payload := mesh.ExecPayload{
			Command: meshExecCommand,
			Args:    meshExecArgs,
			Env:     env,
			WorkDir: meshExecWorkDir,
		}

		payloadData, err := json.Marshal(payload)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal payload")
		}

		command := mesh.Command{
			Type:    mesh.CommandTypeExec,
			Payload: payloadData,
		}

		client := mesh.NewClient(fmt.Sprintf("http://%s", meshExecNode), log.Logger)

		timeout := time.Duration(meshExecTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		result, err := client.ExecuteCommand(ctx, command)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to execute command")
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal result")
		}

		fmt.Println(string(output))
	},
}

var (
	meshFactsNode       string
	meshFactsCategories []string
	meshFactsTimeout    int
)

var meshFactsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Get facts from mesh node",
	Long:  `Get system facts from a specific mesh node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshFactsNode == "" {
			log.Fatal().Msg("node address is required (use --node)")
		}

		payload := mesh.FactsPayload{
			Categories: meshFactsCategories,
		}

		payloadData, err := json.Marshal(payload)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal payload")
		}

		command := mesh.Command{
			Type:    mesh.CommandTypeFacts,
			Payload: payloadData,
		}

		client := mesh.NewClient(fmt.Sprintf("http://%s", meshFactsNode), log.Logger)

		timeout := time.Duration(meshFactsTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		result, err := client.ExecuteCommand(ctx, command)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get facts")
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal result")
		}

		fmt.Println(string(output))
	},
}

var (
	meshApplyNode    string
	meshApplyFiles   []string
	meshApplyDryRun  bool
	meshApplyTimeout int
)

var meshApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply laws on mesh node",
	Long:  `Apply governance laws on a specific mesh node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshApplyNode == "" {
			log.Fatal().Msg("node address is required (use --node)")
		}
		if len(meshApplyFiles) == 0 {
			log.Fatal().Msg("at least one law file is required (use --files)")
		}

		payload := mesh.ApplyLawsPayload{
			LawFiles: meshApplyFiles,
			DryRun:   meshApplyDryRun,
		}

		payloadData, err := json.Marshal(payload)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal payload")
		}

		command := mesh.Command{
			Type:    mesh.CommandTypeApplyLaws,
			Payload: payloadData,
		}

		client := mesh.NewClient(fmt.Sprintf("http://%s", meshApplyNode), log.Logger)

		timeout := time.Duration(meshApplyTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		result, err := client.ExecuteCommand(ctx, command)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to apply laws")
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to marshal result")
		}

		fmt.Println(string(output))
	},
}

func init() {
	meshCmd.AddCommand(statusCmd)
	meshCmd.AddCommand(nodesCmd)
	meshCmd.AddCommand(execCmd)
	meshCmd.AddCommand(meshFactsCmd)
	meshCmd.AddCommand(meshApplyCmd)

	// Status command flags
	statusCmd.Flags().StringVar(&meshStatusNode, "node", "", "HTTP address of mesh node (host:port)")
	statusCmd.MarkFlagRequired("node")

	// Nodes command flags
	nodesCmd.Flags().StringVar(&meshStatusNode, "node", "", "HTTP address of mesh node (host:port)")
	nodesCmd.MarkFlagRequired("node")

	// Exec command flags
	execCmd.Flags().StringVar(&meshExecNode, "node", "", "HTTP address of mesh node (host:port)")
	execCmd.Flags().StringVar(&meshExecCommand, "command", "", "Command to execute")
	execCmd.Flags().StringSliceVar(&meshExecArgs, "args", nil, "Command arguments")
	execCmd.Flags().StringVar(&meshExecWorkDir, "workdir", "", "Working directory")
	execCmd.Flags().StringSliceVar(&meshExecEnv, "env", nil, "Environment variables (KEY=VALUE)")
	execCmd.Flags().IntVar(&meshExecTimeout, "timeout", 30, "Timeout in seconds")
	execCmd.MarkFlagRequired("node")
	execCmd.MarkFlagRequired("command")

	// Facts command flags
	meshFactsCmd.Flags().StringVar(&meshFactsNode, "node", "", "HTTP address of mesh node (host:port)")
	meshFactsCmd.Flags().StringSliceVar(&meshFactsCategories, "categories", nil, "Fact categories to retrieve")
	meshFactsCmd.Flags().IntVar(&meshFactsTimeout, "timeout", 30, "Timeout in seconds")
	meshFactsCmd.MarkFlagRequired("node")

	// Apply command flags
	meshApplyCmd.Flags().StringVar(&meshApplyNode, "node", "", "HTTP address of mesh node (host:port)")
	meshApplyCmd.Flags().StringSliceVar(&meshApplyFiles, "files", nil, "Law files to apply")
	meshApplyCmd.Flags().BoolVar(&meshApplyDryRun, "dry-run", false, "Perform dry run without applying changes")
	meshApplyCmd.Flags().IntVar(&meshApplyTimeout, "timeout", 60, "Timeout in seconds")
	meshApplyCmd.MarkFlagRequired("node")
	meshApplyCmd.MarkFlagRequired("files")
}
