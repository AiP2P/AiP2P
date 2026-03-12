package aip2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	anacrolixdht "github.com/anacrolix/dht/v2"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

type SyncOptions struct {
	StoreRoot    string
	QueuePath    string
	NetPath      string
	ListenAddr   string
	Refs         []string
	PollInterval time.Duration
	Timeout      time.Duration
	Once         bool
	Seed         bool
}

type SyncRef struct {
	Raw      string
	Magnet   string
	InfoHash string
}

type SyncItemResult struct {
	Ref       string `json:"ref"`
	InfoHash  string `json:"infohash,omitempty"`
	ContentDir string `json:"content_dir,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func RunSync(ctx context.Context, opts SyncOptions, logf func(string, ...any)) error {
	store, err := OpenStore(opts.StoreRoot)
	if err != nil {
		return err
	}
	queuePath, err := ensureSyncLayout(store, opts.QueuePath)
	if err != nil {
		return err
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 30 * time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	netCfg, err := LoadNetworkBootstrapConfig(opts.NetPath)
	if err != nil {
		return fmt.Errorf("load network bootstrap config: %w", err)
	}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = store.DataDir
	cfg.Seed = opts.Seed
	cfg.NoDefaultPortForwarding = true
	cfg.DisableAcceptRateLimiting = true
	cfg.DhtStartingNodes = func(network string) anacrolixdht.StartingNodesGetter {
		return func() ([]anacrolixdht.Addr, error) {
			return resolveDHTRouters(network, netCfg.DHTRouters)
		}
	}
	if strings.TrimSpace(opts.ListenAddr) != "" {
		cfg.SetListenAddr(opts.ListenAddr)
	}
	client, err := torrent.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create torrent client: %w", err)
	}
	defer client.Close()
	if err := bootstrapTorrentDHT(client, netCfg.DHTRouters); err != nil && logf != nil {
		logf("bootstrap torrent dht: %v", err)
	}

	libp2pRuntime, err := startLibP2PRuntime(ctx, netCfg)
	if err != nil {
		return err
	}
	defer libp2pRuntime.Close()

	runtime := &syncRuntime{
		store:         store,
		queuePath:     queuePath,
		mode:          syncMode(opts.Once),
		seed:          opts.Seed,
		startedAt:     time.Now().UTC(),
		torrentClient: client,
		libp2p:        libp2pRuntime,
		netCfg:        netCfg,
	}
	if err := runtime.writeStatus(ctx); err != nil && logf != nil {
		logf("write sync status: %v", err)
	}
	if logf != nil {
		logf("sync queue: %s", queuePath)
		if netCfg.Exists {
			logf("network bootstrap file: %s", netCfg.FileName())
			logf("configured DHT routers: %d", len(netCfg.DHTRouters))
			logf("configured libp2p peers: %d", len(netCfg.LibP2PBootstrap))
			logf("configured libp2p rendezvous namespaces: %d", len(netCfg.LibP2PRendezvous))
		} else if strings.TrimSpace(opts.NetPath) != "" {
			logf("network bootstrap file not found: %s", opts.NetPath)
		}
	}

	if opts.Once {
		refs, err := collectSyncRefs(opts.Refs, queuePath)
		if err != nil {
			return err
		}
		runtime.setQueueRefs(len(refs))
		if err := runtime.writeStatus(ctx); err != nil && logf != nil {
			logf("write sync status: %v", err)
		}
		if len(refs) == 0 {
			return errors.New("no magnet or infohash refs found")
		}
		for _, ref := range refs {
			result := syncRef(ctx, client, store, ref, opts.Timeout)
			runtime.recordResult(result)
			if err := runtime.writeStatus(ctx); err != nil && logf != nil {
				logf("write sync status: %v", err)
			}
			if logf != nil {
				logf("%s: %s", result.Status, result.Ref)
				if result.Message != "" {
					logf("  %s", result.Message)
				}
			}
		}
		return nil
	}

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()
	for {
		refs, err := collectSyncRefs(opts.Refs, queuePath)
		if err != nil {
			return err
		}
		runtime.setQueueRefs(len(refs))
		if err := runtime.writeStatus(ctx); err != nil && logf != nil {
			logf("write sync status: %v", err)
		}
		for _, ref := range refs {
			result := syncRef(ctx, client, store, ref, opts.Timeout)
			runtime.recordResult(result)
			if err := runtime.writeStatus(ctx); err != nil && logf != nil {
				logf("write sync status: %v", err)
			}
			if logf != nil {
				logf("%s: %s", result.Status, result.Ref)
				if result.Message != "" {
					logf("  %s", result.Message)
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type syncRuntime struct {
	store         *Store
	queuePath     string
	mode          string
	seed          bool
	startedAt     time.Time
	torrentClient *torrent.Client
	libp2p        *libp2pRuntime
	netCfg        NetworkBootstrapConfig
	activity      SyncActivityStatus
}

func (r *syncRuntime) setQueueRefs(n int) {
	r.activity.QueueRefs = n
}

func (r *syncRuntime) recordResult(result SyncItemResult) {
	now := time.Now().UTC()
	r.activity.LastRef = result.Ref
	r.activity.LastInfoHash = result.InfoHash
	r.activity.LastStatus = result.Status
	r.activity.LastMessage = result.Message
	r.activity.LastEventAt = &now
	switch result.Status {
	case "imported":
		r.activity.Imported++
	case "skipped":
		r.activity.Skipped++
	default:
		r.activity.Failed++
	}
}

func (r *syncRuntime) writeStatus(ctx context.Context) error {
	status := SyncRuntimeStatus{
		StartedAt:    r.startedAt,
		PID:          os.Getpid(),
		StoreRoot:    r.store.Root,
		QueuePath:    r.queuePath,
		Mode:         r.mode,
		Seed:         r.seed,
		SyncActivity: r.activity,
	}
	status.LibP2P = r.libp2p.Status(ctx)
	status.BitTorrentDHT = torrentStatus(r.torrentClient, len(r.netCfg.DHTRouters))
	return writeSyncStatus(r.store, status)
}

func torrentStatus(client *torrent.Client, configuredRouters int) SyncBitTorrentStatus {
	status := SyncBitTorrentStatus{
		Enabled:           len(client.DhtServers()) > 0,
		ConfiguredRouters: configuredRouters,
		Servers:           len(client.DhtServers()),
	}
	for _, server := range client.DhtServers() {
		stats, ok := server.Stats().(anacrolixdht.ServerStats)
		if !ok {
			continue
		}
		status.GoodNodes += stats.GoodNodes
		status.Nodes += stats.Nodes
		status.OutstandingTransactions += stats.OutstandingTransactions
	}
	return status
}

func syncMode(once bool) string {
	if once {
		return "once"
	}
	return "daemon"
}

func resolveDHTRouters(network string, routers []string) ([]anacrolixdht.Addr, error) {
	if len(routers) == 0 {
		return anacrolixdht.GlobalBootstrapAddrs(network)
	}
	out := make([]anacrolixdht.Addr, 0, len(routers))
	seen := make(map[string]struct{})
	for _, raw := range routers {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		host, port, err := net.SplitHostPort(raw)
		if err != nil {
			return nil, fmt.Errorf("parse dht router %q: %w", raw, err)
		}
		addrs, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("resolve dht router %q: %w", raw, err)
		}
		for _, ip := range addrs {
			addr := net.JoinHostPort(ip.String(), port)
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			udpAddr, err := net.ResolveUDPAddr(network, addr)
			if err != nil {
				return nil, fmt.Errorf("resolve udp addr %q: %w", addr, err)
			}
			out = append(out, anacrolixdht.NewAddr(udpAddr))
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return anacrolixdht.GlobalBootstrapAddrs(network)
}

func bootstrapTorrentDHT(client *torrent.Client, routers []string) error {
	addrs, err := resolveRouterUDPAddrs(routers)
	if err != nil {
		return err
	}
	for _, server := range client.DhtServers() {
		for _, addr := range addrs {
			server.Ping(addr)
		}
	}
	return nil
}

func resolveRouterUDPAddrs(routers []string) ([]*net.UDPAddr, error) {
	if len(routers) == 0 {
		return nil, nil
	}
	out := make([]*net.UDPAddr, 0, len(routers))
	seen := make(map[string]struct{})
	for _, raw := range routers {
		host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("parse dht router %q: %w", raw, err)
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("resolve dht router %q: %w", raw, err)
		}
		for _, ip := range ips {
			addr := net.JoinHostPort(ip.String(), port)
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			if err != nil {
				return nil, fmt.Errorf("resolve udp dht router %q: %w", addr, err)
			}
			out = append(out, udpAddr)
		}
	}
	return out, nil
}

func ensureSyncLayout(store *Store, queuePath string) (string, error) {
	syncDir := filepath.Join(store.Root, "sync")
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		return "", err
	}
	queuePath = strings.TrimSpace(queuePath)
	if queuePath == "" {
		queuePath = filepath.Join(syncDir, "magnets.txt")
	}
	if err := os.MkdirAll(filepath.Dir(queuePath), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(queuePath); os.IsNotExist(err) {
		if err := os.WriteFile(queuePath, []byte("# magnet:?xt=urn:btih:...\n"), 0o644); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return queuePath, nil
}

func collectSyncRefs(direct []string, queuePath string) ([]SyncRef, error) {
	seen := make(map[string]struct{})
	out := make([]SyncRef, 0)
	add := func(raw string) error {
		for _, part := range splitCommaRefs(raw) {
			ref, err := ParseSyncRef(part)
			if err != nil {
				return err
			}
			key := ref.Magnet
			if ref.InfoHash != "" {
				key = ref.InfoHash
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, ref)
		}
		return nil
	}
	for _, raw := range direct {
		if err := add(raw); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(queuePath) != "" {
		data, err := os.ReadFile(queuePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for lineNo, rawLine := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
				continue
			}
			ref, err := ParseSyncRef(line)
			if err != nil {
				return nil, fmt.Errorf("queue line %d: %w", lineNo+1, err)
			}
			key := ref.Magnet
			if ref.InfoHash != "" {
				key = ref.InfoHash
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, ref)
		}
	}
	return out, nil
}

func ParseSyncRef(raw string) (SyncRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SyncRef{}, errors.New("empty sync ref")
	}
	if strings.HasPrefix(strings.ToLower(raw), "magnet:?") {
		spec, err := torrent.TorrentSpecFromMagnetUri(raw)
		if err != nil {
			return SyncRef{}, fmt.Errorf("parse magnet: %w", err)
		}
		return SyncRef{
			Raw:      raw,
			Magnet:   raw,
			InfoHash: strings.ToLower(spec.InfoHash.HexString()),
		}, nil
	}
	if isHexInfoHash(raw) {
		infoHash := strings.ToLower(raw)
		return SyncRef{
			Raw:      raw,
			Magnet:   "magnet:?xt=urn:btih:" + infoHash,
			InfoHash: infoHash,
		}, nil
	}
	return SyncRef{}, fmt.Errorf("unsupported sync ref %q", raw)
}

func syncRef(ctx context.Context, client *torrent.Client, store *Store, ref SyncRef, timeout time.Duration) SyncItemResult {
	if ref.InfoHash != "" {
		if _, err := os.Stat(store.TorrentPath(ref.InfoHash)); err == nil {
			return SyncItemResult{
				Ref:      ref.Raw,
				InfoHash: ref.InfoHash,
				Status:   "skipped",
				Message:  "torrent already present in local store",
			}
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	t, err := client.AddMagnet(ref.Magnet)
	if err != nil {
		return SyncItemResult{
			Ref:     ref.Raw,
			Status:  "failed",
			Message: fmt.Sprintf("add magnet: %v", err),
		}
	}

	select {
	case <-runCtx.Done():
		return SyncItemResult{
			Ref:      ref.Raw,
			InfoHash: ref.InfoHash,
			Status:   "failed",
			Message:  "timed out waiting for metadata",
		}
	case <-t.GotInfo():
	}

	infoHash := strings.ToLower(t.InfoHash().HexString())
	t.DownloadAll()

	select {
	case <-runCtx.Done():
		return SyncItemResult{
			Ref:      ref.Raw,
			InfoHash: infoHash,
			Status:   "failed",
			Message:  "timed out waiting for bundle download",
		}
	case <-t.Complete().On():
	}

	contentDir := filepath.Join(store.DataDir, t.Name())
	if _, _, err := LoadMessage(contentDir); err != nil {
		return SyncItemResult{
			Ref:      ref.Raw,
			InfoHash: infoHash,
			Status:   "failed",
			Message:  fmt.Sprintf("validate downloaded bundle: %v", err),
		}
	}
	if err := writeTorrentFile(store.TorrentPath(infoHash), t.Metainfo()); err != nil {
		return SyncItemResult{
			Ref:      ref.Raw,
			InfoHash: infoHash,
			Status:   "failed",
			Message:  fmt.Sprintf("write torrent file: %v", err),
		}
	}
	return SyncItemResult{
		Ref:       ref.Raw,
		InfoHash:  infoHash,
		ContentDir: contentDir,
		Status:    "imported",
		Message:   "bundle downloaded and indexed in local store",
	}
}

func writeTorrentFile(path string, mi metainfo.MetaInfo) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return mi.Write(file)
}

func isHexInfoHash(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func splitCommaRefs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
