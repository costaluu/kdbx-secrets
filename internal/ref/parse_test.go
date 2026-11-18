package ref

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	u := [16]byte{0xAB, 0xCD, 0xEF, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	for in, want := range map[string]Ref{
		"AWS/paper-freeze":          {Path: []string{"AWS", "paper-freeze"}, Field: "Password"},
		"AWS/paper-freeze#UserName": {Path: []string{"AWS", "paper-freeze"}, Field: "UserName"},
		"a#region":                  {Path: []string{"a"}, Field: "region"},
		`Web/a\/b#Password`:         {Path: []string{"Web", "a/b"}, Field: "Password"},
		`Web/a\#b\\c`:               {Path: []string{"Web", `a#b\c`}, Field: "Password"},
		`a#x\#y\/z`:                 {Path: []string{"a"}, Field: "x#y/z"},
		" Web / x ":                 {Path: []string{" Web ", " x "}, Field: "Password"},
		"uuid:ABcdef0102030405060708090a0b0c0d#URL": {ByUUID: true, UUID: u, Field: "URL"},
		"uuid:abcdef0102030405060708090a0b0c0d":     {ByUUID: true, UUID: u, Field: "Password"},
	} {
		got, err := Parse(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v, %v; want %+v", in, got, err, want)
		}
	}
}

func TestParseInvalidReportsPosition(t *testing.T) {
	u := strings.Repeat("a", 32)
	for in, pos := range map[string]string{
		"":                                "position 1",
		"#Password":                       "position 1", // empty path
		"a#":                              "position 3", // '#' without field
		"/a":                              "position 1",
		"a/":                              "position 3",
		"a//b":                            "position 3",
		`a\xb`:                            "position 2",
		`ab\`:                             "position 3",
		"a#b#c":                           "position 4",
		"uuid:":                           "position 6",
		"uuid:abc":                        "position 6",
		"uuid:" + strings.Repeat("g", 32): "position 6",
		"uuid:" + u + "/x":                "position 38",
		"uuid:" + u + "#":                 "position 39",
	} {
		_, err := Parse(in)
		if err == nil || !strings.Contains(err.Error(), pos) {
			t.Errorf("%q: err %v, want %s", in, err, pos)
		}
	}
}
