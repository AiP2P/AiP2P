package aip2p

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func fetchTorrentFallback(ctx context.Context, store *Store, ref SyncRef, lanPeers []string) (string, error) {
	if ref.InfoHash == "" {
		return "", fmt.Errorf("missing infohash for torrent fallback")
	}
	target := store.TorrentPath(ref.InfoHash)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for _, endpoint := range candidateTorrentURLs(ref, lanPeers) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, endpoint)
			_ = resp.Body.Close()
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		file, err := os.Create(target)
		if err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		_, copyErr := file.ReadFrom(resp.Body)
		closeErr := resp.Body.Close()
		fileErr := file.Close()
		if copyErr != nil {
			lastErr = copyErr
			_ = os.Remove(target)
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			_ = os.Remove(target)
			continue
		}
		if fileErr != nil {
			lastErr = fileErr
			_ = os.Remove(target)
			continue
		}
		return target, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no torrent fallback candidates")
	}
	return "", lastErr
}

func candidateTorrentURLs(ref SyncRef, lanPeers []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(host string) {
		host = normalizeTorrentHTTPHost(host)
		if host == "" {
			return
		}
		value := "http://" + net.JoinHostPort(host, "51818") + "/api/torrents/" + ref.InfoHash + ".torrent"
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, host := range lanPeers {
		add(host)
	}
	if strings.TrimSpace(ref.Magnet) != "" {
		if uri, err := url.Parse(ref.Magnet); err == nil {
			for _, raw := range uri.Query()["x.pe"] {
				host, _, err := net.SplitHostPort(raw)
				if err != nil {
					continue
				}
				add(host)
			}
		}
	}
	return out
}

func normalizeTorrentHTTPHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil {
			value = u.Host
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	return strings.TrimSpace(value)
}
