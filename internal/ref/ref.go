// Package ref parses and resolves References (ADR 0004).
// Reserved names: TOTP, attachment:<name>; #otp is the plain TOTP seed attribute.
package ref

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	kp "github.com/tobischo/gokeepasslib/v3"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

// Ref addresses a value. Either Path (root group name excluded) or UUID
// (ByUUID) locates the Entry; Field is the Standard field or Attribute
// customizado name, "Password" when the Reference has no '#'.
type Ref struct {
	Path   []string
	ByUUID bool
	UUID   [16]byte
	Field  string
}

func syntaxErr(s string, pos int, msg string) error {
	return fmt.Errorf("reference %q: %s at position %d", s, msg, pos)
}

// Parse checks syntax only (strict, ADR 0004), so it can run before the
// prompt. Positions are 1-based byte offsets.
func Parse(s string) (Ref, error) {
	r := Ref{Field: "Password"}
	i := 0
	if strings.HasPrefix(s, "uuid:") {
		i = 5
		h := s[i:min(len(s), i+32)]
		d, err := hex.DecodeString(h)
		if len(h) != 32 || err != nil {
			return r, syntaxErr(s, i+1, "uuid: needs exactly 32 hex characters")
		}
		r.ByUUID = true
		copy(r.UUID[:], d)
		i += 32
		if i < len(s) && s[i] != '#' {
			return r, syntaxErr(s, i+1, "expected '#' or end after uuid")
		}
	}
	// parts[0] is the path (split on '/'), parts[1] the field after '#'.
	var segs [][]byte
	var cur []byte
	inField := false
	for ; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 == len(s) {
				return r, syntaxErr(s, i+1, "escape at end of reference")
			}
			i++
			if s[i] != '/' && s[i] != '#' && s[i] != '\\' {
				return r, syntaxErr(s, i, "unknown escape")
			}
			cur = append(cur, s[i])
		case c == '#' && inField:
			return r, syntaxErr(s, i+1, "unescaped '#' in field name")
		case c == '#':
			if !r.ByUUID {
				if len(cur) == 0 {
					return r, syntaxErr(s, i+1, "empty path segment")
				}
				segs = append(segs, cur)
			}
			cur, inField = nil, true
			if i+1 == len(s) {
				return r, syntaxErr(s, i+2, "'#' without field name")
			}
		case c == '/' && !inField:
			if len(cur) == 0 {
				return r, syntaxErr(s, i+1, "empty path segment")
			}
			segs = append(segs, cur)
			cur = nil
		default:
			cur = append(cur, c)
		}
	}
	if inField {
		r.Field = string(cur)
	} else if !r.ByUUID {
		if len(cur) == 0 {
			return r, syntaxErr(s, i+1, "empty path segment")
		}
		segs = append(segs, cur)
	}
	if r.Field == attachmentPrefix {
		return r, syntaxErr(s, len(s)-len(attachmentPrefix)+1, "attachment: needs a name")
	}
	for _, g := range segs {
		r.Path = append(r.Path, string(g))
	}
	return r, nil
}

// String renders the Reference for messages (never the value).
func (r Ref) String() string {
	if r.ByUUID {
		return "uuid:" + hex.EncodeToString(r.UUID[:]) + "#" + r.Field
	}
	return strings.Join(r.Path, "/") + "#" + r.Field
}

// Result is a resolved value plus warnings the caller prints to stderr.
type Result struct {
	Value    string
	Warnings []string
}

type node struct {
	g       *kp.Group
	trashed bool
}

// Resolve returns the value addressed by r. History is never consulted and
// Recycle bin entries are errors. Messages cite the Reference, never the value.
func Resolve(d *db.DB, r Ref, now time.Time) (Result, error) {
	e, err := find(d, r)
	if err != nil {
		return Result{}, err
	}
	reserved := r.Field == "TOTP" || strings.HasPrefix(r.Field, attachmentPrefix)
	v := e.Get(r.Field)
	if reserved && v != nil {
		return Result{}, fmt.Errorf("reference %q: entry has a custom attribute named like the reserved name %q", r, r.Field)
	}
	if reserved {
		return resolveReserved(d, e, r, now)
	}
	if v == nil {
		return Result{}, fmt.Errorf("reference %q: field %q not found", r, r.Field)
	}
	if strings.Contains(v.Value.Content, "{REF:") {
		return Result{}, fmt.Errorf("reference %q: KeePass reference not supported", r)
	}
	res := Result{Value: v.Value.Content}
	if t := e.Times; t.Expires.Bool && t.ExpiryTime != nil && t.ExpiryTime.Time.Before(now) {
		res.Warnings = append(res.Warnings, fmt.Sprintf("reference %q: entry is expired", r))
	}
	return res, nil
}

const attachmentPrefix = "attachment:"

// errNotFound marks a missing group or entry on a path, which Plan may create.
var errNotFound = errors.New("not found")

// resolveReserved handles #TOTP and #attachment:<name>, which never come from
// the entry's attributes.
func resolveReserved(d *db.DB, e *kp.Entry, r Ref, now time.Time) (Result, error) {
	if r.Field == "TOTP" {
		seed := e.GetContent("otp")
		if seed == "" {
			return Result{}, fmt.Errorf("reference %q: entry has no TOTP seed", r)
		}
		code, err := TOTP(seed, now)
		if err != nil {
			return Result{}, fmt.Errorf("reference %q: %w", r, err)
		}
		return Result{Value: code}, nil
	}
	name := strings.TrimPrefix(r.Field, attachmentPrefix)
	for _, b := range e.Binaries {
		if b.Name != name {
			continue
		}
		data, err := d.Attachment(b.Value.ID)
		if err != nil {
			return Result{}, fmt.Errorf("reference %q: reading attachment: %w", r, err)
		}
		return Result{Value: string(data)}, nil
	}
	return Result{}, fmt.Errorf("reference %q: attachment %q not found", r, name)
}

func find(d *db.DB, r Ref) (*kp.Entry, error) {
	if r.ByUUID {
		var walk func(g *kp.Group, trashed bool) (*kp.Entry, bool)
		walk = func(g *kp.Group, trashed bool) (*kp.Entry, bool) {
			trashed = trashed || (!d.RecycleBin.IsZero() && g.UUID == d.RecycleBin)
			for i := range g.Entries {
				if [16]byte(g.Entries[i].UUID) == r.UUID {
					return &g.Entries[i], trashed
				}
			}
			for i := range g.Groups {
				if e, t := walk(&g.Groups[i], trashed); e != nil {
					return e, t
				}
			}
			return nil, false
		}
		e, trashed := walk(d.Root, false)
		if e == nil {
			return nil, fmt.Errorf("reference %q: entry not found", r)
		}
		if trashed {
			return nil, fmt.Errorf("reference %q: entry is in the trash", r)
		}
		return e, nil
	}
	// Groups may repeat names too, so follow every match.
	front := []node{{d.Root, false}}
	for _, name := range r.Path[:len(r.Path)-1] {
		var next []node
		for _, n := range front {
			for i := range n.g.Groups {
				if c := &n.g.Groups[i]; c.Name == name {
					next = append(next, node{c, n.trashed || (!d.RecycleBin.IsZero() && c.UUID == d.RecycleBin)})
				}
			}
		}
		if len(next) == 0 {
			return nil, fmt.Errorf("reference %q: group %q %w", r, name, errNotFound)
		}
		front = next
	}
	title := r.Path[len(r.Path)-1]
	var found []*kp.Entry
	trashed := false
	for _, n := range front {
		for i := range n.g.Entries {
			if n.g.Entries[i].GetTitle() == title {
				found = append(found, &n.g.Entries[i])
				trashed = n.trashed
			}
		}
	}
	switch {
	case len(found) == 0:
		return nil, fmt.Errorf("reference %q: entry %w", r, errNotFound)
	case len(found) > 1:
		var ids []string
		for _, e := range found {
			ids = append(ids, "uuid:"+hex.EncodeToString(e.UUID[:]))
		}
		return nil, fmt.Errorf("reference %q: ambiguous, %d entries share this title; use one of: %s", r, len(found), strings.Join(ids, ", "))
	case trashed:
		return nil, fmt.Errorf("reference %q: entry is in the trash", r)
	}
	return found[0], nil
}
