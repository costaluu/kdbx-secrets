package ref

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	kp "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

func entry(title, pw string, extra ...string) kp.Entry {
	e := kp.NewEntry()
	add := func(k, v string) {
		e.Values = append(e.Values, kp.ValueData{Key: k, Value: kp.V{Content: v}})
	}
	add("Title", title)
	add("Password", pw)
	for i := 0; i < len(extra); i += 2 {
		add(extra[i], extra[i+1])
	}
	return e
}

func group(name string, es []kp.Entry, gs ...kp.Group) kp.Group {
	g := kp.NewGroup()
	g.Name, g.Entries, g.Groups = name, es, gs
	return g
}

func hx(u kp.UUID) string { return hex.EncodeToString(u[:]) }

var now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// fixtureDB: Root{AWS{paper-freeze, dup, dup, " Sp ace", expired}, Recycle Bin{old}}.
func fixtureDB() (d *db.DB, paperFreeze, old kp.UUID) {
	pf := entry("paper-freeze", "s3cr3t", "UserName", "me", "region", "sa-east-1", "Region", "x", "ref", "{REF:P@I:abc}")
	pf.Histories = []kp.History{{Entries: []kp.Entry{entry("paper-freeze", "OLD-HISTORY")}}}
	exp := entry("expired", "pw")
	exp.Times.Expires = w.NewBoolWrapper(true)
	exp.Times.ExpiryTime = &w.TimeWrapper{Time: now.Add(-time.Hour)}
	o := entry("old", "trashed")
	bin := group("Recycle Bin", []kp.Entry{o})
	root := group("Root", nil,
		group("AWS", []kp.Entry{pf, entry("dup", "1"), entry("dup", "2"), entry(" Sp ace", "sp"), exp}),
		bin)
	return &db.DB{Root: &root, RecycleBin: bin.UUID}, pf.UUID, o.UUID
}

func resolve(t *testing.T, d *db.DB, s string) (Result, error) {
	t.Helper()
	r, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return Resolve(d, r, now)
}

func TestResolveOK(t *testing.T) {
	d, pf, _ := fixtureDB()
	u := "uuid:" + strings.ToUpper(hx(pf))
	for in, want := range map[string]string{
		"AWS/paper-freeze":          "s3cr3t",
		"AWS/paper-freeze#Password": "s3cr3t",
		"AWS/paper-freeze#UserName": "me",
		"AWS/paper-freeze#region":   "sa-east-1",
		"AWS/paper-freeze#Region":   "x",  // case-sensitive
		"AWS/ Sp ace":               "sp", // spaces are part of the name
		u:                           "s3cr3t",
		u + "#region":               "sa-east-1",
	} {
		if r, err := resolve(t, d, in); err != nil || r.Value != want {
			t.Errorf("%q: %q, %v; want %q", in, r.Value, err, want)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	d, _, old := fixtureDB()
	for in, want := range map[string]string{
		"aws/paper-freeze":                "group",
		"AWS/Paper-freeze":                "not found",
		"AWS/paper-freeze#nope":           "not found",
		"AWS/paper-freeze#ref":            "KeePass reference not supported",
		"AWS/paper-freeze ":               "not found",
		"AWS/dup":                         "uuid:",
		"Recycle Bin/old":                 "trash",
		"uuid:" + hx(old):                 "trash",
		"uuid:" + strings.Repeat("0", 32): "not found",
	} {
		_, err := resolve(t, d, in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err %v, want %q", in, err, want)
		}
		if err != nil && (strings.Contains(err.Error(), "s3cr3t") || strings.Contains(err.Error(), "REF:P")) {
			t.Errorf("%q: error leaks value: %v", in, err)
		}
	}
	if _, err := resolve(t, d, "AWS/dup"); strings.Count(err.Error(), "uuid:") != 2 {
		t.Errorf("want both candidates: %v", err)
	}
}

func TestResolveExpiredWarns(t *testing.T) {
	d, _, _ := fixtureDB()
	r, err := resolve(t, d, "AWS/expired")
	if err != nil || r.Value != "pw" || len(r.Warnings) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
}
