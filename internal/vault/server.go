// Package vault is the read-only KV v2 imitation behind `run --vault`
// (response shape in docs/spike-kv-v2.md, limits in ADR 0005).
package vault

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

// Secret is one Entry of the Scope. Seed is the TOTP seed ("" = none); it
// is only used to compute TOTP and is never served.
type Secret struct {
	Fields map[string]string
	Seed   string
}

// Scope maps "<mount>/<path>" to its Entry.
type Scope map[string]Secret

// Server is a running listener on 127.0.0.1. Token lives only in memory.
type Server struct {
	Addr  string // host:port
	Token string
	srv   *http.Server
}

// Close stops the listener.
func (s *Server) Close() error { return s.srv.Close() }

// Start serves scope on a random 127.0.0.1 port. now is the TOTP clock.
func Start(scope Scope, now func() time.Time) (*Server, error) {
	tok := make([]byte, 32) // 256 bits
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{Token: hex.EncodeToString(tok)}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.Addr = ln.Addr().String()
	s.srv = &http.Server{
		Handler:           handler(scope, s.Token, now),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
	}
	go s.srv.Serve(ln)
	return s, nil
}

func handler(scope Scope, token string, now func() time.Time) http.Handler {
	mounts := map[string]bool{}
	for k := range scope {
		m, _, _ := strings.Cut(k, "/")
		mounts[m] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			fail(w, 405, "method not allowed")
			return
		}
		if !authorized(r, token) {
			fail(w, 403, "permission denied")
			return
		}
		// The query is ignored and the body is never read.
		raw := r.URL.EscapedPath()
		if strings.Contains(strings.ToLower(raw), "%2f") {
			fail(w, 400, "invalid path")
			return
		}
		// A single trailing '/' (or the bare root) is just an unknown path: 404 below.
		if trimmed := strings.TrimSuffix(strings.TrimPrefix(raw, "/"), "/"); trimmed != "" {
			for _, seg := range strings.Split(trimmed, "/") {
				if seg == ".." || seg == "." || seg == "" || strings.ContainsAny(seg, "%\\") {
					fail(w, 400, "invalid path")
					return
				}
			}
		}
		p, ok := strings.CutPrefix(r.URL.Path, "/v1/")
		if !ok {
			fail(w, 404, "")
			return
		}
		if ui, ok := strings.CutPrefix(p, "sys/internal/ui/mounts/"); ok {
			// The vault CLI asks this before kv get; it must say "kv v2".
			m, _, _ := strings.Cut(ui, "/")
			if !mounts[m] {
				fail(w, 404, "")
				return
			}
			reply(w, map[string]any{"path": m + "/", "type": "kv", "options": map[string]any{"version": "2"}})
			return
		}
		m, rest, _ := strings.Cut(p, "/")
		path, ok := strings.CutPrefix(rest, "data/")
		sec, found := scope[m+"/"+path]
		if !ok || !found {
			fail(w, 404, "")
			return
		}
		data := make(map[string]string, len(sec.Fields)+1)
		for k, v := range sec.Fields {
			data[k] = v
		}
		if sec.Seed != "" { // no seed (or an unusable one) means no TOTP
			if code, err := ref.TOTP(sec.Seed, now()); err == nil {
				data["TOTP"] = code
			}
		}
		reply(w, map[string]any{"data": map[string]any{"data": data, "metadata": map[string]any{
			"created_time": "2018-03-22T02:24:06.945319214Z", "custom_metadata": nil,
			"deletion_time": "", "destroyed": false, "version": 1,
		}}})
	})
}

func authorized(r *http.Request, token string) bool {
	got := r.Header.Get("X-Vault-Token")
	if b, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && got == "" {
		got = b
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// reply wraps data in the standard Vault envelope (docs/spike-kv-v2.md).
func reply(w http.ResponseWriter, body map[string]any) {
	body["request_id"], body["lease_id"], body["renewable"], body["lease_duration"] = "00000000-0000-0000-0000-000000000000", "", false, 0
	body["wrap_info"], body["warnings"], body["auth"] = nil, nil, nil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, code int, msg string) {
	errs := []string{}
	if msg != "" {
		errs = append(errs, msg)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"errors": errs})
}
