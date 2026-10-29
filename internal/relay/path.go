package relay

import (
	"strings"

	"github.com/rejisterjack/plumb/internal/protocol"
)

// SplitHookPath maps a request URI onto a tunnel slug and the path forwarded
// to localhost. "/h/{slug}/webhooks/stripe?x=1" becomes slug "{slug}" and
// rest "/webhooks/stripe?x=1". The slug prefix is removed.
func SplitHookPath(requestURI string) (slug, rest string, ok bool) {
	pathPart, query, hasQuery := strings.Cut(requestURI, "?")
	if !strings.HasPrefix(pathPart, "/h/") {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(pathPart, "/h/")
	slug, after, found := strings.Cut(trimmed, "/")
	if !protocol.ValidSlug(slug) {
		return "", "", false
	}
	rest = "/"
	if found && after != "" {
		rest = "/" + after
	}
	if hasQuery {
		rest += "?" + query
	}
	return slug, rest, true
}
