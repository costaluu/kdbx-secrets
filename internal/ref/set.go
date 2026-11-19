package ref

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

// Writable are the fields keepassxc-cli add/edit can set (ADR 0006).
var Writable = map[string]bool{"UserName": true, "Password": true, "URL": true, "Notes": true}

// Gap is what adapter fill must do for one Reference.
type Gap struct {
	Missing   bool     // the value must be asked and written
	NewEntry  bool     // the Entry does not exist
	NewGroups []string // group paths to create first, parents first
}

// Plan reports what is missing for r without changing anything. A value set
// already is never a Gap; a missing value keepassxc-cli cannot write is an error.
func Plan(d *db.DB, r Ref, now time.Time) (Gap, error) {
	e, err := find(d, r)
	if err != nil && !errors.Is(err, errNotFound) {
		return Gap{}, err
	}
	if e != nil {
		if r.Field == "Title" {
			return Gap{}, nil
		}
		if res, err := Resolve(d, r, now); err == nil && res.Value != "" {
			return Gap{}, nil
		}
	}
	if !Writable[r.Field] && !(e == nil && r.Field == "Title") {
		return Gap{}, fmt.Errorf("reference %q: keepassxc-cli cannot write this field, add it in KeePassXC", r)
	}
	if r.ByUUID { // keepassxc-cli addresses entries by path only
		return Gap{}, fmt.Errorf("reference %q: uuid references cannot be filled, use the path", r)
	}
	if e != nil {
		return Gap{Missing: true}, nil
	}
	// First-match walk, like KeePassXC's findGroupByPath.
	g := d.Root
	groups := r.Path[:len(r.Path)-1]
	i := 0
	for ; i < len(groups); i++ {
		var next = -1
		for j := range g.Groups {
			if g.Groups[j].Name == groups[i] {
				next = j
				break
			}
		}
		if next < 0 {
			break
		}
		g = &g.Groups[next]
	}
	gap := Gap{Missing: r.Field != "Title", NewEntry: true}
	for ; i < len(groups); i++ {
		gap.NewGroups = append(gap.NewGroups, strings.Join(groups[:i+1], "/"))
	}
	return gap, nil
}
