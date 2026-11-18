//go:build unix

package cli_test

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

// Helper-process modes, dispatched before TestMain runs the tests.
func init() {
	switch os.Getenv("KS_HELPER") {
	case "term":
		termChild()
	case "pdeath-parent":
		pdeathParent()
	case "pdeath-child":
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)
		<-ch
		os.WriteFile(os.Getenv("KS_MARK"), nil, 0o600)
		os.Exit(0)
	}
}

// termChild waits for SIGTERM, then still reads the server before exiting.
func termChild() {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	fmt.Println("ready")
	for s := range ch {
		if s == syscall.SIGINT {
			fmt.Println("got-sigint")
			continue
		}
		req, _ := http.NewRequest("GET", os.Getenv("VAULT_ADDR")+"/v1/secret/data/aws", nil)
		req.Header.Set("X-Vault-Token", os.Getenv("VAULT_TOKEN"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("server-error", err)
			os.Exit(3)
		}
		fmt.Println("server", resp.StatusCode)
		os.Exit(0)
	}
}

// pdeathParent starts a child bound with cli.Bind and waits to be killed.
func pdeathParent() {
	runtime.LockOSThread()
	c := exec.Command(os.Args[0])
	c.Env = []string{"KS_HELPER=pdeath-child", "KS_MARK=" + os.Getenv("KS_MARK")}
	cli.Bind(c)
	if err := c.Start(); err != nil {
		fmt.Println("start-error", err)
		os.Exit(1)
	}
	fmt.Println("ready")
	select {}
}

type syncBuf struct {
	mu sync.Mutex
	sb strings.Builder
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.Write(p)
}
func (b *syncBuf) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.sb.String() }

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timeout waiting for %s", what)
}

func TestRunForwardsSIGTERMAndIgnoresSIGINT(t *testing.T) {
	h := newHarness(t, fixture("derived.kdbx"))
	out := &syncBuf{}
	code := make(chan int, 1)
	go func() {
		code <- runOut(t, h, "term", out, "run", "--vault", "secret/aws=AWS/with-totp", "--", os.Args[0])
	}()
	waitFor(t, "child ready", func() bool { return strings.Contains(out.String(), "ready") })
	syscall.Kill(os.Getpid(), syscall.SIGINT) // must not kill run (nor reach the child)
	time.Sleep(200 * time.Millisecond)
	select {
	case c := <-code:
		t.Fatalf("run exited on SIGINT: %d", c)
	default:
	}
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
	if c := <-code; c != 0 {
		t.Fatalf("exit %d: %s", c, h.errw.String())
	}
	if o := out.String(); !strings.Contains(o, "server 200") || strings.Contains(o, "got-sigint") {
		t.Fatalf("output: %s", o)
	}
}

func TestChildGetsSIGTERMWhenParentIsKilled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Pdeathsig is Linux only")
	}
	mark := t.TempDir() + "/mark"
	c := exec.Command(os.Args[0])
	c.Env = []string{"KS_HELPER=pdeath-parent", "KS_MARK=" + mark}
	p, _ := c.StdoutPipe()
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if l, _ := bufio.NewReader(p).ReadString('\n'); l != "ready\n" {
		t.Fatalf("parent: %q", l)
	}
	time.Sleep(200 * time.Millisecond) // let the grandchild install its handler
	c.Process.Kill()
	c.Wait()
	waitFor(t, "grandchild SIGTERM", func() bool { _, err := os.Stat(mark); return err == nil })
}

func TestRunChildKilledBySignalExits128Plus(t *testing.T) {
	h := newHarness(t, fixture("derived.kdbx"))
	if code := runWith(t, h, "selfkill", "run", "--vault", "secret/aws=AWS/with-totp", "--", os.Args[0]); code != 128+9 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
}
