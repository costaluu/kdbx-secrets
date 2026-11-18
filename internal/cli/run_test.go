package cli_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

// TestMain re-executes the test binary as the `run` child (helper process).
func TestMain(m *testing.M) {
	switch os.Getenv("KS_HELPER") {
	case "probe":
		probe()
		os.Exit(0)
	case "exit7":
		os.Exit(7)
	case "selfkill":
		p, _ := os.FindProcess(os.Getpid())
		p.Kill()
		select {}
	}
	os.Exit(m.Run())
}

// probe prints "<label> <status> <body>" per scenario, then the environment.
func probe() {
	base, tok := "http://"+strings.TrimPrefix(os.Getenv("VAULT_ADDR"), "http://"), os.Getenv("VAULT_TOKEN")
	do := func(label, method, path, header, val string) {
		req, _ := http.NewRequest(method, base+path, nil)
		if header != "" {
			req.Header.Set(header, val)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(label, "ERR", err)
			return
		}
		b, _ := io.ReadAll(resp.Body)
		fmt.Println(label, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	do("in", "GET", "/v1/secret/data/aws", "X-Vault-Token", tok)
	do("bearer", "GET", "/v1/secret/data/aws", "Authorization", "Bearer "+tok)
	do("out", "GET", "/v1/secret/data/other", "X-Vault-Token", tok)
	do("notoken", "GET", "/v1/secret/data/aws", "", "")
	do("badtoken", "GET", "/v1/secret/data/aws", "X-Vault-Token", "nope")
	do("put", "PUT", "/v1/secret/data/aws", "X-Vault-Token", tok)
	do("post", "POST", "/v1/secret/data/aws", "X-Vault-Token", tok)
	do("query", "GET", "/v1/secret/data/aws?version=3", "X-Vault-Token", tok)
	do("mounts", "GET", "/v1/sys/internal/ui/mounts/secret/aws", "X-Vault-Token", tok)
	do("noseed", "GET", "/v1/secret/data/plain", "X-Vault-Token", tok)
	for _, p := range []string{"/v1/secret/data/aws/../plain", "/v1/secret/data/aws%2Fx", "/v1/secret/data/aws%2fx", "/v1/secret/data/./aws", "/v1/secret/data/aws/", "/v1/secret/data//aws"} {
		c, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
		if err != nil {
			fmt.Println("raw ERR", err)
			continue
		}
		fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: x\r\nX-Vault-Token: %s\r\nConnection: close\r\n\r\n", p, tok)
		line, _ := bufio.NewReader(c).ReadString('\n')
		fmt.Println("raw", p, strings.TrimSpace(line))
		c.Close()
	}
	for _, e := range os.Environ() {
		fmt.Println("env", e)
	}
}

func runWith(t *testing.T, h *harness, helper string, args ...string) int {
	t.Helper()
	return runOut(t, h, helper, &h.stdout, args...)
}

func runOut(t *testing.T, h *harness, helper string, out io.Writer, args ...string) int {
	t.Helper()
	return cli.Run(&cli.Env{
		Args:   args,
		Getenv: func(k string) string { return map[string]string{"KDBX_SECRETS_HOME": h.home}[k] },
		Stdout: out,
		Stderr: &h.errw,
		OpenTerminal: func() (cli.Terminal, error) {
			h.terminalOpens++
			h.termR = strings.NewReader(h.termIn)
			return fakeTerm{h}, nil
		},
		Now:     func() time.Time { return time.Unix(59, 0) },
		Environ: func() []string { return []string{"PATH=" + os.Getenv("PATH"), "KS_HELPER=" + helper} },
	})
}

func TestRunVaultServer(t *testing.T) {
	h := newHarness(t, fixture("derived.kdbx"))
	code := runWith(t, h, "probe", "run",
		"--vault", "secret/aws=AWS/with-totp", "--vault", "secret/plain=AWS/no-totp",
		"--", os.Args[0])
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	out := h.stdout.String()
	line := func(prefix string) string {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, prefix+" ") {
				return l
			}
		}
		t.Fatalf("no %q line in %s", prefix, out)
		return ""
	}
	for label, status := range map[string]string{"in": "200", "bearer": "200", "query": "200", "mounts": "200",
		"out": "404", "notoken": "403", "badtoken": "403", "put": "405", "post": "405"} {
		if l := line(label); !strings.HasPrefix(l, label+" "+status+" ") {
			t.Errorf("%s: want %s, got %s", label, status, l)
		}
	}
	if l := line("mounts"); !strings.Contains(l, `"path":"secret/"`) || !strings.Contains(l, `"version":"2"`) {
		t.Errorf("mounts: %s", l)
	}
	for _, p := range []string{"/v1/secret/data/aws/../plain", "/v1/secret/data/aws%2Fx", "/v1/secret/data/aws%2fx", "/v1/secret/data/./aws"} {
		want := " 400 "
		if strings.HasSuffix(p, "aws/") {
			want = " 404 " // trailing '/' is an unknown path, not a malformed one
		}
		if l := line("raw " + p); !strings.Contains(l, want) {
			t.Errorf("path %s not refused: %s", p, l)
		}
	}
	in := line("in")
	if !regexp.MustCompile(`"TOTP":"287082"`).MatchString(in) || !strings.Contains(in, `"data":{"data":{`) {
		t.Errorf("TOTP missing: %s", in)
	}
	if strings.Contains(in, "otpauth") || strings.Contains(in, "GEZDGN") || strings.Contains(in, "attachment") || strings.Contains(in, "chave") {
		t.Errorf("seed or attachment leaked: %s", in)
	}
	if l := line("noseed"); strings.Contains(l, "TOTP") {
		t.Errorf("TOTP without seed: %s", l)
	}
	// Environment: only VAULT_*, nothing from the Database.
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "env ") && (strings.Contains(l, "GEZDGN") || strings.Contains(l, "fixture-pass") || strings.Contains(l, "s3cr3t")) {
			t.Errorf("secret in child env: %s", l)
		}
	}
	if !strings.Contains(out, "env VAULT_ADDR=http://127.0.0.1:") || !strings.Contains(out, "env VAULT_TOKEN=") {
		t.Errorf("VAULT_ADDR/VAULT_TOKEN missing")
	}
	if h.terminalOpens != 1 {
		t.Errorf("prompts: %d", h.terminalOpens)
	}
}

func TestRunPropagatesChildExitCode(t *testing.T) {
	h := newHarness(t, fixture("derived.kdbx"))
	if code := runWith(t, h, "exit7", "run", "--vault", "secret/aws=AWS/with-totp", "--", os.Args[0]); code != 7 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
}

func TestRunMappingErrorsExitTwoBeforePrompt(t *testing.T) {
	for _, args := range [][]string{
		{"run", "--", "x"},
		{"run", "--vault", "secret/a=AWS/x"},
		{"run", "--vault", "secret/a", "--", "x"},
		{"run", "--vault", "secret/a=AWS/x", "--vault", "secret/a=AWS/y", "--", "x"},
		{"run", "--vault", "sys/a=AWS/x", "--", "x"},
		{"run", "--vault", "auth/a=AWS/x", "--", "x"},
		{"run", "--vault", "identity/a=AWS/x", "--", "x"},
		{"run", "--vault", "cubbyhole/a=AWS/x", "--", "x"},
		{"run", "--vault", "secret//a=AWS/x", "--", "x"},
		{"run", "--vault", "secret/./a=AWS/x", "--", "x"},
		{"run", "--vault", "secret/../a=AWS/x", "--", "x"},
		{"run", "--vault", "secret/a=AWS/x#UserName", "--", "x"},
		{"run", "--vault", "secret/a=a//b", "--", "x"},
	} {
		h := newHarness(t, fixture("derived.kdbx"))
		if code := runWith(t, h, "", args...); code != 2 || h.terminalOpens != 0 {
			t.Errorf("%v: exit %d opens %d err %s", args, code, h.terminalOpens, h.errw.String())
		}
	}
}

// Optional: the real client reads the secret; skipped without `vault`.
func TestRunWithRealVaultCLI(t *testing.T) {
	bin, err := exec.LookPath("vault")
	if err != nil {
		t.Skip("vault CLI not installed")
	}
	h := newHarness(t, fixture("derived.kdbx"))
	code := runWith(t, h, "", "run", "--vault", "secret/aws=AWS/with-totp", "--", bin, "kv", "get", "-field=UserName", "secret/aws")
	if code != 0 || h.stdout.Len() == 0 {
		t.Fatalf("exit %d out %q err %s", code, h.stdout.String(), h.errw.String())
	}
}
