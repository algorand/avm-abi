package apps

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeBoxKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		description string
		name        string
		app         uint64
		key         string
		err         string
	}

	pp := func(tc testCase) string {
		return fmt.Sprintf("<<<%s>>> (name, app) = (%#v, %d) --should--> key = %#v (err = [%s])", tc.description, tc.name, tc.app, tc.key, tc.err)
	}

	var testCases = []testCase{
		// COPACETIC:
		{"zero appid", "stranger", 0, "bx:\x00\x00\x00\x00\x00\x00\x00\x00stranger", ""},
		{"typical", "348-8uj", 131231, "bx:\x00\x00\x00\x00\x00\x02\x00\x9f348-8uj", ""},
		{"empty box name", "", 42, "bx:\x00\x00\x00\x00\x00\x00\x00*", ""},
		{"random byteslice", "{\xbb\x04\a\xd1\xe2\xc6I\x81{", 13475904583033571713, "bx:\xbb\x04\a\xd1\xe2\xc6I\x81{\xbb\x04\a\xd1\xe2\xc6I\x81{", ""},

		// ERRORS:
		{"too short", "", 0, "stranger", "SplitBoxKey() cannot extract AppIndex as key (stranger) too short (length=8)"},
		{"wrong prefix", "", 0, "strangersINTHEdark", "SplitBoxKey() illegal app box prefix in key (strangersINTHEdark). Expected prefix 'bx:'"},
	}

	for _, tc := range testCases {
		app, name, err := SplitBoxKey(tc.key)

		if tc.err == "" {
			key := MakeBoxKey(uint64(tc.app), tc.name)
			require.Equal(t, uint64(tc.app), app, pp(tc))
			require.Equal(t, tc.name, name, pp(tc))
			require.Equal(t, tc.key, key, pp(tc))
		} else {
			require.EqualError(t, err, tc.err, pp(tc))
		}
	}
}
