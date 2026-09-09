package logs

import (
	"net"
	"net/http"
	"net/netip"
	"server/config"
	"strings"
)

func getRemoteAddr(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	peer = peer.Unmap()
	// Our reverse proxy overwrites X-Real-IP. Never trust client-supplied chains.
	for _, cidr := range strings.Split(config.TrustedProxies, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err == nil && prefix.Contains(peer) {
			if address, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
				return address.Unmap().String()
			}
			break
		}
	}
	return peer.String()
}
