package vdf_test

import (
	"bytes"
	"fmt"
	"testing"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	type config struct {
		Name    string            `vdf:"name"`
		Enabled bool              `vdf:"enabled"`
		Ratio   float64           `vdf:"ratio"`
		Note    string            `vdf:"note,omitempty"`
		Ptr     *int              `vdf:"ptr"`
		Ignored string            `vdf:"-"`
		Depots  map[int]depot     `vdf:"depots"`
		Tags    map[string]string `vdf:"tags,omitempty"`
	}

	actual, err := vdf.Marshal(&struct{ Config config }{config{
		Name:    "a \"quoted\"\tpath\\",
		Enabled: true,
		Ratio:   0.5,
		Ignored: "x",
		Depots:  map[int]depot{10: {1, 2}, 2: {3, 4}, -1: {5, 6}},
	}})
	require.NoError(t, err)

	expected := `"Config"
{
	"name"		"a \"quoted\"\tpath\\"
	"enabled"		"1"
	"ratio"		"0.5"
	"depots"
	{
		"-1"
		{
			"manifest"		"5"
			"size"		"6"
		}
		"2"
		{
			"manifest"		"3"
			"size"		"4"
		}
		"10"
		{
			"manifest"		"1"
			"size"		"2"
		}
	}
}
`
	require.Equal(t, expected, string(actual))
}

func TestMarshalErrors(t *testing.T) {
	type cycle struct{ Next *cycle }
	c := &cycle{}
	c.Next = c

	for _, tc := range []struct {
		name string
		v    any
	}{
		{"top-level string", "top-level string"},
		{"nil", nil},
		{"nil pointer", (*struct{})(nil)},
		{"slice", struct{ S []string }{S: []string{"a"}}},
		{"float map key", map[float64]string{1: "a"}},
		{"cycle", c},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vdf.Marshal(tc.v)
			require.Error(t, err)
		})
	}

	t.Run("unsupported type", func(t *testing.T) {
		var unsupported *vdf.UnsupportedTypeError
		_, err := vdf.Marshal(struct{ C chan int }{make(chan int)})
		require.ErrorAs(t, err, &unsupported)
	})
}

func TestRoundTrip(t *testing.T) {
	var expected struct{ AppState *appState }
	require.NoError(t, vdf.Unmarshal([]byte(appManifest), &expected))

	var buf bytes.Buffer
	require.NoError(t, vdf.NewEncoder(&buf).Encode(expected))

	var actual struct{ AppState *appState }
	require.NoError(t, vdf.NewDecoder(&buf).Decode(&actual))
	require.Equal(t, expected, actual)

	for i := range maxInt {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			expected := map[string]any{randString(): randString(), "nested": map[string]any{randString(): "\"\\\n\t" + randString()}}

			b, err := vdf.Marshal(expected)
			require.NoError(t, err)

			var actual map[string]any
			require.NoError(t, vdf.Unmarshal(b, &actual))
			require.Equal(t, expected, actual)
		})
	}
}
