package sandbox

import (
	"net"
	"strings"
)

// HostAllowed reports whether host is permitted by allowed.
// Entries may be exact hostnames or "*.example.com" wildcards.
// An empty allowlist denies all hosts. Port suffixes are stripped.
func HostAllowed(host string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "*.") {
			suf := a[1:] // ".example.com"
			if strings.HasSuffix(host, suf) || host == a[2:] {
				return true
			}
			continue
		}
		if host == a {
			return true
		}
	}
	return false
}
