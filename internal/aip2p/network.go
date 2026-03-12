package aip2p

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

type NetworkBootstrapConfig struct {
	Path             string
	Exists           bool
	NetworkID        string
	DHTRouters       []string
	LibP2PBootstrap  []string
	LibP2PRendezvous []string
}

func LoadNetworkBootstrapConfig(path string) (NetworkBootstrapConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return NetworkBootstrapConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NetworkBootstrapConfig{Path: path}, nil
		}
		return NetworkBootstrapConfig{}, err
	}
	cfg := NetworkBootstrapConfig{
		Path:   path,
		Exists: true,
	}
	seenDHT := make(map[string]struct{})
	seenLibP2P := make(map[string]struct{})
	seenRendezvous := make(map[string]struct{})
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "network_id":
			if cfg.NetworkID == "" {
				cfg.NetworkID = normalizeNetworkID(value)
			}
		case "dht_router":
			if _, ok := seenDHT[value]; ok {
				continue
			}
			seenDHT[value] = struct{}{}
			cfg.DHTRouters = append(cfg.DHTRouters, value)
		case "libp2p_bootstrap":
			if _, ok := seenLibP2P[value]; ok {
				continue
			}
			seenLibP2P[value] = struct{}{}
			cfg.LibP2PBootstrap = append(cfg.LibP2PBootstrap, value)
		case "libp2p_rendezvous", "rendezvous":
			if _, ok := seenRendezvous[value]; ok {
				continue
			}
			seenRendezvous[value] = struct{}{}
			cfg.LibP2PRendezvous = append(cfg.LibP2PRendezvous, value)
		}
	}
	return cfg, nil
}

func (c NetworkBootstrapConfig) FileName() string {
	if c.Path == "" {
		return ""
	}
	return filepath.Base(c.Path)
}

func normalizeNetworkID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}
