// Copyright © 2026 Iggy <iggy@theiggy.com>
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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/iggy/govern/pkg/mesh"
)

var (
	meshListenAddrs   []string
	meshBootstrap     []string
	meshRendezvous    string
	meshControlAddr   string
	meshDataDir       string
	meshPrivate       bool
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mesh node",
	Long: `Start a govern mesh node.

The mesh is masterless: there is no leader, no quorum, and no shared cluster
state. Nodes discover each other over mDNS (LAN) and a Kademlia DHT (WAN),
traverse NAT via hole punching and relays, and communicate over encrypted
libp2p streams. Any node with a routable address can serve as a bootstrap/relay
peer for NAT'd nodes – such peers hold no authority over the mesh.

Examples:
  # First/seed node on a public address (others bootstrap from it)
  govern mesh start --listen=/ip4/0.0.0.0/tcp/63001 --listen=/ip4/0.0.0.0/udp/63001/quic-v1

  # A node that bootstraps from a known peer (note the /p2p/<peerid> suffix)
  govern mesh start --bootstrap=/ip4/203.0.113.10/udp/63001/quic-v1/p2p/12D3Koo...

  # A NAT'd node that should reserve relay slots
  govern mesh start --bootstrap=/ip4/203.0.113.10/udp/63001/quic-v1/p2p/12D3Koo... --private`,
	Run: func(cmd *cobra.Command, args []string) {
		if meshDataDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatal().Err(err).Msg("failed to get home directory")
			}
			meshDataDir = filepath.Join(homeDir, ".govern", "mesh-data")
		}
		if err := os.MkdirAll(meshDataDir, 0o700); err != nil {
			log.Fatal().Err(err).Str("dir", meshDataDir).Msg("failed to create data directory")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cfg := mesh.Config{
			ListenAddrs:    meshListenAddrs,
			BootstrapPeers: meshBootstrap,
			Rendezvous:     meshRendezvous,
			DataDir:        meshDataDir,
			Private:        meshPrivate,
		}

		service, err := mesh.NewService(ctx, cfg, log.Logger)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create mesh service")
		}

		if err := service.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("failed to start mesh service")
		}

		// Local control socket: the CLI talks to this node over loopback, and
		// the node fans requests out across the mesh. Bind to localhost.
		httpServer := mesh.NewHTTPServer(service, meshControlAddr, log.Logger)
		go func() {
			if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("control socket error")
			}
		}()

		log.Info().
			Str("control", meshControlAddr).
			Str("data_dir", meshDataDir).
			Str("rendezvous", meshRendezvous).
			Bool("private", meshPrivate).
			Msg("mesh node started")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("shutting down mesh node")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := httpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("failed to stop control socket")
		}
		if err := service.Stop(); err != nil {
			log.Error().Err(err).Msg("failed to stop mesh service")
		}
	},
}

func init() {
	meshCmd.AddCommand(startCmd)

	startCmd.Flags().StringSliceVar(&meshListenAddrs, "listen", nil, "libp2p listen multiaddrs (default: TCP+QUIC on all interfaces, random port)")
	startCmd.Flags().StringSliceVar(&meshBootstrap, "bootstrap", nil, "Bootstrap/relay peer multiaddrs (e.g. /ip4/.../udp/63001/quic-v1/p2p/12D3Koo...)")
	startCmd.Flags().StringVar(&meshRendezvous, "rendezvous", "govern-mesh", "Rendezvous string; nodes sharing it form one mesh")
	startCmd.Flags().StringVar(&meshControlAddr, "control", "127.0.0.1:8008", "Local control socket address for the CLI")
	startCmd.Flags().StringVar(&meshDataDir, "data-dir", "", "Data directory (holds the persistent node identity) (default: ~/.govern/mesh-data)")
	startCmd.Flags().BoolVar(&meshPrivate, "private", false, "Hint that this node is behind NAT (reserve relay slots)")

	viper.BindPFlag("mesh.listen", startCmd.Flags().Lookup("listen"))
	viper.BindPFlag("mesh.bootstrap", startCmd.Flags().Lookup("bootstrap"))
	viper.BindPFlag("mesh.rendezvous", startCmd.Flags().Lookup("rendezvous"))
	viper.BindPFlag("mesh.control", startCmd.Flags().Lookup("control"))
	viper.BindPFlag("mesh.data-dir", startCmd.Flags().Lookup("data-dir"))
	viper.BindPFlag("mesh.private", startCmd.Flags().Lookup("private"))
}
