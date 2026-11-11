package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const maxDisplayBody = 64 << 10

// LogExchange prints one webhook the way a developer reads it in the terminal.
// note is an optional error string printed under the summary line.
func LogExchange(w io.Writer, replay bool, method, urlPath string, status int, dur time.Duration, headers http.Header, body []byte, note string) {
	if w == nil {
		return
	}
	var b strings.Builder
	tag := ""
	if replay {
		tag = "replay "
	}
	fmt.Fprintf(&b, "%s%s %s -> %d (%s)\n", tag, sanitizeLog(method), sanitizeLog(urlPath), status, dur.Round(time.Millisecond))
	if note != "" {
		fmt.Fprintf(&b, "error: %s\n", sanitizeLog(note))
	}
	if len(headers) > 0 {
		keys := make([]string, 0, len(headers))
		for k := range headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range headers.Values(k) {
				fmt.Fprintf(&b, "%s: %s\n", sanitizeLog(k), sanitizeLog(v))
			}
		}
	}
	b.WriteByte('\n')
	b.WriteString(displayBody(body))
	b.WriteString("----\n")
	_, _ = io.WriteString(w, b.String())
}

func displayBody(b []byte) string {
	if len(b) == 0 {
		return "(empty body)\n"
	}
	if !utf8.Valid(b) {
		return fmt.Sprintf("(binary %d bytes)\n", len(b))
	}
	if len(b) <= maxDisplayBody {
		return sanitizeLog(string(b)) + trailingNewline(b)
	}
	return sanitizeLog(string(b[:maxDisplayBody])) + "\n… truncated\n"
}

func trailingNewline(b []byte) string {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return ""
	}
	return "\n"
}

// sanitizeLog strips ASCII control characters so a webhook body cannot
// rewrite the terminal. Newlines and tabs are kept. The bytes forwarded to
// localhost are not passed through this function.
func sanitizeLog(s string) string {
	if s == "" {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return '\uFFFD'
		}
		return r
	}, s)
}
