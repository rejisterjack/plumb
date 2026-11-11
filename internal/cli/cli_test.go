package cli

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rejisterjack/plumb/internal/relay"
)

func TestLoadFileConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".plumb.toml")
	body := "# comment\nrelay = \"https://relay.example\"\npath = 'alice'\nport = 3000\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay != "https://relay.example" || cfg.Path != "alice" || cfg.Port != 3000 {
		t.Fatalf("cfg = %+v", cfg)
	}
	missing, err := loadFileConfig(filepath.Join(dir, "nope.toml"))
	if err != nil || missing != (fileConfig{}) {
		t.Fatalf("missing = %+v %v", missing, err)
	}
}

func TestConnectFlagsOverrideFile(t *testing.T) {
	t.Setenv("PLUMB_TOKEN", "from-env")
	file := fileConfig{Relay: "https://file.example", Path: "fromfile", Port: 9}
	opts, err := parseConnect([]string{"--port", "3000"}, file)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Relay != "https://file.example" || opts.Path != "fromfile" || opts.Port != 3000 || opts.Token != "from-env" {
		t.Fatalf("opts = %+v", opts)
	}
	over, err := parseConnect([]string{"--relay", "https://flag.example", "--path", "flag", "--token", "flag-token"}, file)
	if err != nil {
		t.Fatal(err)
	}
	if over.Relay != "https://flag.example" || over.Path != "flag" || over.Token != "flag-token" || over.Port != 9 {
		t.Fatalf("over = %+v", over)
	}
	if _, err := parseConnect(nil, fileConfig{}); err == nil {
		t.Fatal("expected missing relay error")
	}
}

func TestResolveListenAddr(t *testing.T) {
	t.Setenv("PORT", "")
	if resolveListenAddr("") != ":8080" {
		t.Fatalf("default addr")
	}
	t.Setenv("PORT", "9090")
	if resolveListenAddr("") != ":9090" {
		t.Fatal(resolveListenAddr(""))
	}
	t.Setenv("PORT", "127.0.0.1:9091")
	if resolveListenAddr("") != "127.0.0.1:9091" {
		t.Fatal(resolveListenAddr(""))
	}
	if resolveListenAddr(":7") != ":7" {
		t.Fatal("flag should win")
	}
}

func TestParseRelayRequiresToken(t *testing.T) {
	t.Setenv("PLUMB_TOKEN", "")
	if _, err := parseRelay(nil); err == nil {
		t.Fatal("expected token error")
	}
	opts, err := parseRelay([]string{"--dev"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Dev || opts.Addr != ":8080" {
		t.Fatalf("opts = %+v", opts)
	}
	t.Setenv("PLUMB_TOKEN", "s3cret")
	opts, err = parseRelay(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Token != "s3cret" {
		t.Fatalf("token = %q", opts.Token)
	}
}

func TestParseReplayDefaults(t *testing.T) {
	opts, err := parseReplay(nil, fileConfig{Relay: "https://relay.example", Path: "abc", Port: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Last != 5 || opts.Path != "abc" || opts.Port != 3000 {
		t.Fatalf("opts = %+v", opts)
	}
	if _, err := parseReplay([]string{"--last", "0"}, fileConfig{}); err == nil {
		t.Fatal("expected last error")
	}
}

func TestMainUsage(t *testing.T) {
	if Main(nil) != 2 {
		t.Fatal("no args")
	}
	if Main([]string{"help"}) != 0 {
		t.Fatal("help")
	}
	if Main([]string{"nope"}) != 2 {
		t.Fatal("unknown")
	}
}

func TestVersionOutput(t *testing.T) {
	code, out, errOut := captureOutput(t, func() int {
		return Main([]string{"version"})
	})
	if code != 0 {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	if out != "plumb "+version+"\n" {
		t.Fatalf("version output = %q", out)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestReplayRejectsBadToken(t *testing.T) {
	const relayToken = "expected-secret"
	const given = "wrong-token"
	srv := relay.New(relay.Config{Token: relayToken}, nil)
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	code, out, errOut := captureOutput(t, func() int {
		return Main([]string{
			"replay",
			"--last", "1",
			"--relay", ts.URL,
			"--path", "qabadtoken",
			"--port", "9",
			"--token", given,
		})
	})
	combined := out + errOut
	if code != 1 {
		t.Fatalf("exit %d output %s", code, combined)
	}
	if !strings.Contains(errOut, "unauthorized") {
		t.Fatalf("stderr = %q", errOut)
	}
	if strings.Contains(combined, relayToken) || strings.Contains(combined, given) {
		t.Fatalf("token leaked: %s", combined)
	}
}

func captureOutput(t *testing.T, fn func() int) (int, string, string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	code := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = origOut, origErr
	out, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(out), string(errOut)
}
