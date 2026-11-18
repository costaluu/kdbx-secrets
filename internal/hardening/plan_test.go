package hardening

import "testing"

const MiB = 1 << 20

func TestPlan(t *testing.T) {
	inf := ^uint64(0)
	need := func(kdf uint64) uint64 { return kdf + Margin }
	cases := []struct {
		name            string
		kdf, soft, hard uint64
		lock            bool
		raiseTo         uint64
	}{
		{"soft already covers", 64 * MiB, 512 * MiB, 512 * MiB, true, 0},
		{"raise soft to need", 64 * MiB, 8 * MiB, 1024 * MiB, true, need(64 * MiB)},
		{"hard exactly covers", 64 * MiB, 8 * MiB, need(64 * MiB), true, need(64 * MiB)},
		{"hard too low", 64 * MiB, 8 * MiB, 8 * MiB, false, 0},
		{"hard one byte short", 64 * MiB, 8 * MiB, need(64*MiB) - 1, false, 0},
		{"unlimited soft", 64 * MiB, inf, inf, true, 0},
		{"unlimited hard raises soft", 64 * MiB, 8 * MiB, inf, true, need(64 * MiB)},
		{"AES-KDF needs only margin", 0, Margin, Margin, true, 0},
		{"AES-KDF below margin", 0, 8 * MiB, 8 * MiB, false, 0},
		{"overflow does not wrap", inf, inf - 1, inf - 1, false, 0},
	}
	for _, c := range cases {
		d := Plan(c.kdf, c.soft, c.hard)
		if d.Lock != c.lock || d.RaiseTo != c.raiseTo {
			t.Errorf("%s: got %+v, want lock=%v raiseTo=%d", c.name, d, c.lock, c.raiseTo)
		}
		if !d.Lock && d.Warning == "" {
			t.Errorf("%s: no warning when not locking", c.name)
		}
	}
}
