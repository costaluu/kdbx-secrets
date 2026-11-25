package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const redactMark = "*****"

// redactValues picks what `run` hides from the child's output: every Password
// of 8+ bytes, plus any value with a high-entropy alphanumeric run (16+ chars,
// >= 3.5 bits/char, as in sigillo), so "us-east-1" or a host stays readable.
// Each value also counts in its JSON-escaped forms (keys in `-format=json`).
func redactValues(fields []map[string]string) []string {
	seen := map[string]bool{}
	for _, f := range fields {
		for k, v := range f {
			if (k == "Password" && len(v) >= 8) || likelySecret(v) {
				seen[v] = true
				for _, html := range []bool{true, false} {
					var b bytes.Buffer
					e := json.NewEncoder(&b)
					e.SetEscapeHTML(html)
					e.Encode(v)
					seen[strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(b.String(), `"`), "\n"), `"`)] = true
				}
			}
		}
	}
	var out []string
	for v := range seen {
		out = append(out, v)
	}
	// strings.Replacer tries the pairs in order at each position: longest wins.
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func likelySecret(v string) bool {
	for _, seg := range strings.FieldsFunc(v, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if len(seg) >= 16 && entropy(seg) >= 3.5 {
			return true
		}
	}
	return false
}

func entropy(s string) float64 {
	var n [256]int
	for i := 0; i < len(s); i++ {
		n[s[i]]++
	}
	h := 0.0
	for _, c := range n {
		if c > 0 {
			p := float64(c) / float64(len(s))
			h -= p * math.Log2(p)
		}
	}
	return h
}

// redactor replaces the values in a stream. A tail that may be the start of a
// value is held back; if nothing else arrives within idle, it is written
// as is (a prompt waiting for input must not stall).
type redactor struct {
	mu   sync.Mutex
	w    io.Writer
	vals []string
	rep  *strings.Replacer
	held []byte
	t    *time.Timer
}

const redactIdle = 100 * time.Millisecond

func newRedactor(w io.Writer, vals []string) *redactor {
	pairs := make([]string, 0, 2*len(vals))
	for _, v := range vals {
		pairs = append(pairs, v, redactMark)
	}
	return &redactor{w: w, vals: vals, rep: strings.NewReplacer(pairs...)}
}

func (r *redactor) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.t != nil {
		r.t.Stop()
	}
	s := r.rep.Replace(string(r.held) + string(p))
	k := r.partial(s)
	r.held = []byte(s[len(s)-k:])
	if k > 0 {
		r.t = time.AfterFunc(redactIdle, func() { r.Close() })
	}
	if _, err := io.WriteString(r.w, s[:len(s)-k]); err != nil {
		return 0, err
	}
	return len(p), nil
}

// partial is the length of the longest tail of s that starts some value.
func (r *redactor) partial(s string) int {
	for k := min(len(s), len(r.vals[0])-1); k > 0; k-- {
		for _, v := range r.vals {
			if strings.HasPrefix(v, s[len(s)-k:]) {
				return k
			}
		}
	}
	return 0
}

// Close writes what is held back (the stream ended or went idle).
func (r *redactor) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.t != nil {
		r.t.Stop()
	}
	_, err := r.w.Write(r.held)
	r.held = nil
	return err
}
