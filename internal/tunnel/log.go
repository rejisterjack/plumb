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
	fmt.Fprintf(&b, "%s%s %s -> %d (%s)\n", tag, method, urlPath, status, dur.Round(time.Millisecond))
	if note != "" {
		fmt.Fprintf(&b, "error: %s\n", note)
	}
	if len(headers) > 0 {
		keys := make([]string, 0, len(headers))
		for k := range headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range headers.Values(k) {
				fmt.Fprintf(&b, "%s: %s\n", k, v)
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
		s := string(b)
		if strings.HasSuffix(s, "\n") {
			return s
		}
		return s + "\n"
	}
	return string(b[:maxDisplayBody]) + "\n… truncated\n"
}
