package ref

import (
	"fmt"
	"strings"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

// Snapshot copies what `run --vault` may serve for the Entry r addresses:
// every Standard field and Custom attribute, except the TOTP seed, which is
// returned apart so the caller can compute TOTP per request. Attachments are never
// included. r.Field is ignored. Errors cite the Reference, never a value.
func Snapshot(d *db.DB, r Ref) (fields map[string]string, seed string, err error) {
	e, err := find(d, r)
	if err != nil {
		return nil, "", err
	}
	fields = map[string]string{}
	for _, v := range e.Values {
		switch {
		case v.Key == "otp":
			seed = v.Value.Content
		case v.Key == "TOTP" || strings.HasPrefix(v.Key, attachmentPrefix):
			return nil, "", fmt.Errorf("reference %q: entry has a custom attribute named like the reserved name %q", r, v.Key)
		case strings.Contains(v.Value.Content, "{REF:"):
			return nil, "", fmt.Errorf("reference %q: KeePass reference not supported", r)
		default:
			fields[v.Key] = v.Value.Content
		}
	}
	return fields, seed, nil
}
