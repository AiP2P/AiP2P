package aip2p

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type SyncSubscriptions struct {
	Channels []string `json:"channels"`
	Topics   []string `json:"topics"`
	Tags     []string `json:"tags"`
}

func LoadSyncSubscriptions(path string) (SyncSubscriptions, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SyncSubscriptions{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SyncSubscriptions{}, nil
		}
		return SyncSubscriptions{}, err
	}
	var rules SyncSubscriptions
	if err := json.Unmarshal(data, &rules); err != nil {
		return SyncSubscriptions{}, err
	}
	rules.Normalize()
	return rules, nil
}

func (r *SyncSubscriptions) Normalize() {
	if r == nil {
		return
	}
	r.Channels = uniqueFold(r.Channels)
	r.Topics = uniqueFold(r.Topics)
	r.Tags = uniqueFold(r.Tags)
}

func (r SyncSubscriptions) Empty() bool {
	return len(r.Channels) == 0 && len(r.Topics) == 0 && len(r.Tags) == 0
}

func uniqueFold(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func containsFold(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}
