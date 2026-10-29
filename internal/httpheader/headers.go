// Package httpheader copies HTTP headers for proxying without hop-by-hop fields.
package httpheader

import (
	"net/http"
	"strings"
)

// CloneForward returns a copy of h without hop-by-hop headers, Host, or
// Content-Length. The local HTTP client sets those itself.
func CloneForward(h http.Header) http.Header {
	hop := map[string]struct{}{
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"Proxy-Connection":    {},
		"Te":                  {},
		"Trailers":            {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
		"Host":                {},
		"Content-Length":      {},
	}
	if h != nil {
		if c := h.Get("Connection"); c != "" {
			for part := range strings.SplitSeq(c, ",") {
				name := http.CanonicalHeaderKey(strings.TrimSpace(part))
				if name != "" {
					hop[name] = struct{}{}
				}
			}
		}
	}
	out := make(http.Header)
	for k, vs := range h {
		if _, skip := hop[http.CanonicalHeaderKey(k)]; skip {
			continue
		}
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}
