package aip2p

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSyncRefMagnet(t *testing.T) {
	t.Parallel()

	ref, err := ParseSyncRef("magnet:?xt=urn:btih:93a71a010a59022c8670e06e2c92fa279f98d974&dn=test")
	if err != nil {
		t.Fatalf("ParseSyncRef error = %v", err)
	}
	if ref.InfoHash != "93a71a010a59022c8670e06e2c92fa279f98d974" {
		t.Fatalf("infohash = %q", ref.InfoHash)
	}
}

func TestParseSyncRefInfoHash(t *testing.T) {
	t.Parallel()

	ref, err := ParseSyncRef("93a71a010a59022c8670e06e2c92fa279f98d974")
	if err != nil {
		t.Fatalf("ParseSyncRef error = %v", err)
	}
	if ref.Magnet != "magnet:?xt=urn:btih:93a71a010a59022c8670e06e2c92fa279f98d974" {
		t.Fatalf("magnet = %q", ref.Magnet)
	}
}

func TestCollectSyncRefsQueueAndDirect(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	queue := filepath.Join(root, "magnets.txt")
	data := "# comment\n93a71a010a59022c8670e06e2c92fa279f98d974\nmagnet:?xt=urn:btih:93a71a010a59022c8670e06e2c92fa279f98d974&dn=test\n"
	if err := os.WriteFile(queue, []byte(data), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}
	refs, err := collectSyncRefs([]string{"90498b9d42e081acee4165af5f5a2554b5276cbb"}, queue)
	if err != nil {
		t.Fatalf("collect refs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("refs len = %d, want 2", len(refs))
	}
}

func TestLoadNetworkBootstrapConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "aip2p_net.inf")
	content := `network_id=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
dht_router=router.bittorrent.com:6881
dht_router=router.utorrent.com:6881
lan_peer=192.168.102.74
libp2p_bootstrap=/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write net config: %v", err)
	}
	cfg, err := LoadNetworkBootstrapConfig(path)
	if err != nil {
		t.Fatalf("load network config: %v", err)
	}
	if len(cfg.DHTRouters) != 2 {
		t.Fatalf("dht routers = %d, want 2", len(cfg.DHTRouters))
	}
	if len(cfg.LibP2PBootstrap) != 1 {
		t.Fatalf("libp2p peers = %d, want 1", len(cfg.LibP2PBootstrap))
	}
	if cfg.NetworkID == "" {
		t.Fatal("expected network id to load")
	}
	if len(cfg.LANPeers) != 1 {
		t.Fatalf("lan peers = %d, want 1", len(cfg.LANPeers))
	}
}

func TestLANBootstrapEndpointDefaultsToLatestPort(t *testing.T) {
	t.Parallel()

	value, err := lanBootstrapEndpoint("192.168.102.74")
	if err != nil {
		t.Fatalf("lanBootstrapEndpoint error = %v", err)
	}
	if value != "http://192.168.102.74:51818/api/network/bootstrap" {
		t.Fatalf("endpoint = %q", value)
	}
}

func TestRemoveSyncRef(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	queue := filepath.Join(root, "magnets.txt")
	content := "# magnet:?xt=urn:btih:...\nmagnet:?xt=urn:btih:93a71a010a59022c8670e06e2c92fa279f98d974&dn=test\n"
	if err := os.WriteFile(queue, []byte(content), 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}
	ref, err := ParseSyncRef("93a71a010a59022c8670e06e2c92fa279f98d974")
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	if err := removeSyncRef(queue, ref); err != nil {
		t.Fatalf("remove ref: %v", err)
	}
	data, err := os.ReadFile(queue)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if string(data) != "# magnet:?xt=urn:btih:...\n" {
		t.Fatalf("queue contents = %q", string(data))
	}
}
