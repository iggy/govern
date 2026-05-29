package mesh

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

// mdnsServiceTag is the LAN discovery rendezvous tag. Nodes advertising the
// same tag on the local segment find each other without any configuration.
const mdnsServiceTag = "govern-mesh"

// HostConfig configures the libp2p networking layer for a govern node. There is
// deliberately no notion of replica IDs, initial members, or join semantics –
// nodes are peers, discovered dynamically, with no coordinator holding cluster
// state. The only "infrastructure" is the optional set of bootstrap/relay
// peers, which hold no authority and exist purely to help NAT'd nodes find and
// reach each other.
type HostConfig struct {
	// ListenAddrs are the multiaddrs to listen on. If empty, sensible
	// defaults covering TCP and QUIC on all interfaces are used.
	ListenAddrs []string
	// BootstrapPeers are multiaddrs of well-known peers used to seed the DHT
	// and the initial connection graph. Any node with a routable address can
	// serve as one; they carry no special authority.
	BootstrapPeers []string
	// Rendezvous is the DHT advertisement string. All nodes sharing a
	// rendezvous string form one logical mesh. Defaults to "govern-mesh".
	Rendezvous string
	// DataDir is where the node's persistent identity key is stored so the
	// PeerID is stable across restarts.
	DataDir string
	// ForceReachabilityPrivate hints that this node is behind a NAT and should
	// proactively reserve relay slots (AutoRelay) and attempt hole punching.
	// When false (the default) libp2p auto-detects reachability via AutoNAT.
	ForceReachabilityPrivate bool
}

// Host wraps a libp2p host together with the discovery machinery that keeps the
// mesh's peer set populated.
type Host struct {
	host       host.Host
	dht        *dht.IpfsDHT
	disc       *drouting.RoutingDiscovery
	rendezvous string
	logger     zerolog.Logger

	mu    sync.RWMutex
	peers map[peer.ID]peer.AddrInfo
}

// NewHost builds and starts a libp2p host with encryption (Noise + TLS 1.3),
// QUIC and TCP transports, NAT traversal (AutoNAT, hole punching, AutoRelay),
// and both LAN (mDNS) and WAN (Kademlia DHT) peer discovery.
func NewHost(ctx context.Context, cfg HostConfig, logger zerolog.Logger) (*Host, error) {
	logger = logger.With().Str("component", "mesh-host").Logger()

	if cfg.Rendezvous == "" {
		cfg.Rendezvous = mdnsServiceTag
	}

	privKey, err := loadOrCreateIdentity(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to set up identity: %w", err)
	}

	listenAddrs := cfg.ListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/tcp/0",
			"/ip6/::/udp/0/quic-v1",
		}
	}

	bootstrapInfos, err := parsePeers(cfg.BootstrapPeers)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap peer: %w", err)
	}

	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(listenAddrs...),
		// Encryption in transit is mandatory: TLS 1.3 preferred, Noise as a
		// fallback. There is no unencrypted path.
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultTransports,
		// NAT traversal: advertise observed addresses, attempt UPnP/NAT-PMP
		// port mapping, and enable DCUtR hole punching for direct NAT-to-NAT
		// connections brokered (without a central signaling server) by relays.
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableHolePunching(),
		// AutoRelay lets NAT'd nodes reserve slots on relay-capable peers so
		// they remain reachable until a direct connection is hole-punched.
		libp2p.EnableAutoRelayWithPeerSource(
			func(ctx context.Context, num int) <-chan peer.AddrInfo {
				return staticPeerSource(bootstrapInfos, num)
			},
		),
		// Any node with a routable address can relay for NAT'd peers. This is
		// what keeps the mesh masterless: relay duty is distributed, not vested
		// in one coordinator.
		libp2p.EnableRelayService(),
	}

	if cfg.ForceReachabilityPrivate {
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	m := &Host{
		host:       h,
		rendezvous: cfg.Rendezvous,
		logger:     logger,
		peers:      make(map[peer.ID]peer.AddrInfo),
	}

	// The DHT is what makes WAN discovery masterless: peers advertise and
	// resolve the rendezvous key through the distributed hash table rather than
	// registering with a central directory.
	kdht, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto), dht.BootstrapPeers(bootstrapInfos...))
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}
	if err := kdht.Bootstrap(ctx); err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}
	m.dht = kdht
	m.disc = drouting.NewRoutingDiscovery(kdht)

	m.connectBootstrap(ctx, bootstrapInfos)

	if err := m.startMDNS(); err != nil {
		logger.Warn().Err(err).Msg("mDNS discovery unavailable")
	}

	logger.Info().
		Str("peer_id", h.ID().String()).
		Strs("addrs", multiaddrsToStrings(h.Addrs())).
		Str("rendezvous", cfg.Rendezvous).
		Msg("libp2p host started")

	return m, nil
}

// ID returns this node's stable peer identity.
func (m *Host) ID() peer.ID { return m.host.ID() }

// FindPeer resolves a peer's addresses via the DHT. Used to reach the origin of
// a broadcast (to return results) when no direct connection exists yet.
func (m *Host) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return m.dht.FindPeer(ctx, id)
}

// Host exposes the underlying libp2p host for protocol wiring.
func (m *Host) Host() host.Host { return m.host }

// Addrs returns the multiaddrs this node is reachable on.
func (m *Host) Addrs() []multiaddr.Multiaddr { return m.host.Addrs() }

// Peers returns the currently known mesh peers (excluding self).
func (m *Host) Peers() []peer.AddrInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]peer.AddrInfo, 0, len(m.peers))
	for _, ai := range m.peers {
		out = append(out, ai)
	}
	return out
}

// Advertise announces this node under the rendezvous key and starts a
// background loop that continually re-advertises and discovers peers. It
// returns once the first discovery pass has been scheduled.
func (m *Host) Advertise(ctx context.Context) {
	dutil.Advertise(ctx, m.disc, m.rendezvous)

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			m.discoverOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// discoverOnce performs a single DHT lookup for peers sharing the rendezvous
// key and dials any new ones, recording them in the peer set.
func (m *Host) discoverOnce(ctx context.Context) {
	peerChan, err := m.disc.FindPeers(ctx, m.rendezvous)
	if err != nil {
		m.logger.Debug().Err(err).Msg("peer discovery failed")
		return
	}
	for p := range peerChan {
		if p.ID == m.host.ID() || len(p.Addrs) == 0 {
			continue
		}
		if m.host.Network().Connectedness(p.ID) != 1 { // 1 == network.Connected
			dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := m.host.Connect(dialCtx, p); err != nil {
				m.logger.Debug().Err(err).Str("peer", p.ID.String()).Msg("failed to connect to discovered peer")
				cancel()
				continue
			}
			cancel()
		}
		m.recordPeer(p)
	}
}

func (m *Host) recordPeer(p peer.AddrInfo) {
	m.mu.Lock()
	m.peers[p.ID] = p
	m.mu.Unlock()
	m.logger.Debug().Str("peer", p.ID.String()).Msg("discovered mesh peer")
}

func (m *Host) connectBootstrap(ctx context.Context, infos []peer.AddrInfo) {
	var wg sync.WaitGroup
	for _, info := range infos {
		wg.Add(1)
		go func(info peer.AddrInfo) {
			defer wg.Done()
			dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if err := m.host.Connect(dialCtx, info); err != nil {
				m.logger.Warn().Err(err).Str("peer", info.ID.String()).Msg("failed to connect to bootstrap peer")
				return
			}
			m.recordPeer(info)
		}(info)
	}
	wg.Wait()
}

// Close shuts down discovery and the libp2p host.
func (m *Host) Close() error {
	if m.dht != nil {
		_ = m.dht.Close()
	}
	return m.host.Close()
}

// mdnsNotifee bridges libp2p's mDNS discovery callbacks into the peer set.
type mdnsNotifee struct {
	h *Host
}

func (n *mdnsNotifee) HandlePeerFound(p peer.AddrInfo) {
	if p.ID == n.h.host.ID() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := n.h.host.Connect(ctx, p); err != nil {
		n.h.logger.Debug().Err(err).Str("peer", p.ID.String()).Msg("failed to connect to mDNS peer")
		return
	}
	n.h.recordPeer(p)
}

func (m *Host) startMDNS() error {
	svc := mdns.NewMdnsService(m.host, mdnsServiceTag, &mdnsNotifee{h: m})
	return svc.Start()
}

// loadOrCreateIdentity returns the node's persistent Ed25519 private key,
// generating and saving one on first run so the PeerID is stable across
// restarts. The PeerID derived from this key is the node's cryptographic
// identity – the basis for masterless authentication.
func loadOrCreateIdentity(dataDir string) (crypto.PrivKey, error) {
	if dataDir == "" {
		// Ephemeral identity (new PeerID every start). Fine for one-off CLIs.
		priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
		return priv, err
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	keyPath := filepath.Join(dataDir, "identity.key")

	if data, err := os.ReadFile(keyPath); err == nil {
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal identity key: %w", err)
		}
		return priv, nil
	}

	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity key: %w", err)
	}
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal identity key: %w", err)
	}
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write identity key: %w", err)
	}
	return priv, nil
}

// parsePeers turns "/ip4/.../p2p/Qm..." multiaddr strings into AddrInfos.
func parsePeers(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, a := range addrs {
		if a == "" {
			continue
		}
		ma, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", a, err)
		}
		info, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", a, err)
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// staticPeerSource feeds a fixed set of relay-capable peers to AutoRelay.
func staticPeerSource(infos []peer.AddrInfo, num int) <-chan peer.AddrInfo {
	ch := make(chan peer.AddrInfo, len(infos))
	n := 0
	for _, info := range infos {
		if n >= num {
			break
		}
		ch <- info
		n++
	}
	close(ch)
	return ch
}

func multiaddrsToStrings(addrs []multiaddr.Multiaddr) []string {
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}
