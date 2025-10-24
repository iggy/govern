// Copyright © 2020 Iggy <iggy@theiggy.com>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors
//    may be used to endorse or promote products derived from this software
//    without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/iggy/govern/pkg/mesh"
)

var (
	meshReplicaID      uint64
	meshRaftAddress    string
	meshHTTPAddress    string
	meshDataDir        string
	meshInitialMembers []string
	meshJoin           bool
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mesh",
	Long: `Start the mesh node with the specified configuration.

Examples:
  # Start first node (creates cluster)
  govern mesh start --replica-id=1 --raft-address=localhost:63001 --http-address=localhost:8001

  # Start additional nodes (joins existing cluster)
  govern mesh start --replica-id=2 --raft-address=localhost:63002 --http-address=localhost:8002 --join --initial-members=1=localhost:63001

  # Start with custom data directory
  govern mesh start --replica-id=1 --raft-address=localhost:63001 --http-address=localhost:8001 --data-dir=/var/lib/govern`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshReplicaID == 0 {
			log.Fatal().Msg("replica-id is required")
		}
		if meshRaftAddress == "" {
			log.Fatal().Msg("raft-address is required")
		}
		if meshHTTPAddress == "" {
			log.Fatal().Msg("http-address is required")
		}

		initialMembers := make(map[uint64]string)
		if !meshJoin {
			initialMembers[meshReplicaID] = meshRaftAddress
		} else {
			for _, member := range meshInitialMembers {
				parts := strings.Split(member, "=")
				if len(parts) != 2 {
					log.Fatal().Str("member", member).Msg("invalid initial member format, expected id=address")
				}
				id, err := strconv.ParseUint(parts[0], 10, 64)
				if err != nil {
					log.Fatal().Str("member", member).Err(err).Msg("invalid replica ID in initial member")
				}
				initialMembers[id] = parts[1]
			}
		}

		if meshDataDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get home directory")
			}
			meshDataDir = filepath.Join(homeDir, ".govern", "mesh-data")
		}

		if err := os.MkdirAll(meshDataDir, 0755); err != nil {
			log.Fatal().Err(err).Str("dir", meshDataDir).Msg("failed to create data directory")
		}

		cfg := mesh.Config{
			ReplicaID:      meshReplicaID,
			RaftAddress:    meshRaftAddress,
			DataDir:        meshDataDir,
			InitialMembers: initialMembers,
			Join:           meshJoin,
		}

		service, err := mesh.NewService(cfg, log.Logger)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create mesh service")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := service.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("failed to start mesh service")
		}

		httpServer := mesh.NewHTTPServer(service, meshHTTPAddress, log.Logger)
		go func() {
			if err := httpServer.Start(); err != nil {
				log.Error().Err(err).Msg("HTTP server error")
			}
		}()

		log.Info().
			Uint64("replica_id", meshReplicaID).
			Str("raft_address", meshRaftAddress).
			Str("http_address", meshHTTPAddress).
			Str("data_dir", meshDataDir).
			Bool("join", meshJoin).
			Msg("mesh node started")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("shutting down mesh node")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := httpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("failed to stop HTTP server")
		}

		if err := service.Stop(); err != nil {
			log.Error().Err(err).Msg("failed to stop mesh service")
		}
	},
}

func init() {
	meshCmd.AddCommand(startCmd)

	startCmd.Flags().Uint64Var(&meshReplicaID, "replica-id", 0, "Unique replica ID for this node")
	startCmd.Flags().StringVar(&meshRaftAddress, "raft-address", "", "Raft address for this node (host:port)")
	startCmd.Flags().StringVar(&meshHTTPAddress, "http-address", "", "HTTP API address for this node (host:port)")
	startCmd.Flags().StringVar(&meshDataDir, "data-dir", "", "Data directory for mesh storage (default: ~/.govern/mesh-data)")
	startCmd.Flags().StringSliceVar(&meshInitialMembers, "initial-members", nil, "Initial cluster members in format id=address (required when joining)")
	startCmd.Flags().BoolVar(&meshJoin, "join", false, "Join existing cluster instead of creating new one")

	startCmd.MarkFlagRequired("replica-id")
	startCmd.MarkFlagRequired("raft-address")
	startCmd.MarkFlagRequired("http-address")

	viper.BindPFlag("mesh.replica-id", startCmd.Flags().Lookup("replica-id"))
	viper.BindPFlag("mesh.raft-address", startCmd.Flags().Lookup("raft-address"))
	viper.BindPFlag("mesh.http-address", startCmd.Flags().Lookup("http-address"))
	viper.BindPFlag("mesh.data-dir", startCmd.Flags().Lookup("data-dir"))
	viper.BindPFlag("mesh.initial-members", startCmd.Flags().Lookup("initial-members"))
	viper.BindPFlag("mesh.join", startCmd.Flags().Lookup("join"))
}
