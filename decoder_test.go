//nolint:gosec
package vdf_test

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/netip"
	"strings"
	"testing"
	"time"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/stretchr/testify/require"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

const (
	letters    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lenLetters = len(letters)
	maxInt     = 16
	delta      = 0.0001
)

func randString() string {
	var (
		n = rnd.Intn(maxInt) + 1
		b = make([]byte, n)
	)
	for i := range b {
		b[i] = letters[rnd.Intn(lenLetters)]
	}

	return string(b)
}

const appManifest = "\xEF\xBB\xBF" + `// written by Steam
"AppState"
{
	"appid"		"10"
	"name"		"Counter-Strike"
	"StateFlags"		"4"
	"installdir"	"Half-Life\\cstrike"
	"quote"		"say \"hi\"\n"
	"winpath"	"C:\Program Files"
	unquoted	bare_value
	"platform"	"win"	[$WIN32]
	"InstalledDepots"
	{
		"1"
		{
			"manifest"		"3101776004287380455"
			"size"		"12"
		}
		"2"
		{
			"manifest"		"18446744073709551615"
			"size"		"34"
		}
	}
	"UserConfig"
	{
		"language"		"english"
	}
}
`

type depot struct {
	Manifest uint64 `vdf:"manifest"`
	Size     int    `vdf:"size"`
}

type appState struct {
	AppID           int              `vdf:"appid"`
	Name            string           `vdf:"name"`
	StateFlags      uint8            // matched case-sensitively
	InstallDir      string           // matched case-insensitively
	Quote           string           `vdf:"quote"`
	WinPath         string           `vdf:"winpath"`
	Unquoted        string           `vdf:"unquoted"`
	Platform        string           `vdf:"platform"`
	InstalledDepots map[uint32]depot `vdf:"InstalledDepots"`
	UserConfig      map[string]any   `vdf:"UserConfig"`
}

func TestDecode(t *testing.T) {
	for i := range maxInt {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var (
				string1 = randString()
				string2 = randString()
				string3 = randString()
				key1    = randString()
				string4 = randString()
				string5 = randString()
				string6 = randString()
				int1    = rnd.Int()
				int2    = int8(rnd.Intn(math.MaxUint8+1) + math.MinInt8)
				int3    = int16(rnd.Intn(math.MaxUint16+1) + math.MinInt16)
				int4    = rnd.Int31() - rnd.Int31()
				int5    = rnd.Int63() - rnd.Int63()
				float1  = rnd.Float32()
				float2  = rnd.Float64()
				key2    = randString()
				string7 = randString()
				data    = fmt.Appendf(nil, `{
		"Key1"  "%s"
		"key2"  "%s"
		"Key3"  {
			"Key4" "%s"
		}
		"Key5"	{
			"%s" 	"%s"
		}
		"Key7"	{
			"Key8" "%s"
		}
		"Key9"	{
			"key10" 		"%s"
		}
		"Key11" ""
		"Key12" "%d"
		"Key13" "%d"
		"Key14" "%d"
		"Key15" "%d"
		"Key16" "%d"
		"Key17" "1"
		"Key18" "%f"
		"Key19" "%f"
		"Key20" {
			"%s" 	{
				"key22" "%s"
			}
		}
	}`, string1, string2, string3, key1, string4, string5, string6, int1, int2, int3, int4, int5, float1, float2, key2, string7)
				actual struct {
					Key1 string
					Key2 string `vdf:"key2"`
					Key3 struct{ Key4 string }
					Key5 map[string]any
					Key9 *struct {
						Key10 string `vdf:"key10"`
					}
					Key11 string
					Key12 int
					Key13 int8
					Key14 int16
					Key15 int32
					Key16 int64
					Key17 bool
					Key18 float32
					Key19 float64
					Key20 map[string]struct {
						Key22 string `vdf:"key22"`
					}
				}
			)

			require.NoError(t, vdf.NewDecoder(strings.NewReader(string(data))).Decode(&actual))
			require.Equal(t, string1, actual.Key1)
			require.Equal(t, string2, actual.Key2)
			require.Equal(t, string3, actual.Key3.Key4)
			require.Equal(t, map[string]any{key1: string4}, actual.Key5)
			require.NotNil(t, actual.Key9)
			require.Equal(t, string6, actual.Key9.Key10)
			require.Empty(t, actual.Key11)
			require.Equal(t, int1, actual.Key12)
			require.Equal(t, int2, actual.Key13)
			require.Equal(t, int3, actual.Key14)
			require.Equal(t, int4, actual.Key15)
			require.Equal(t, int5, actual.Key16)
			require.True(t, actual.Key17)
			require.InDelta(t, float1, actual.Key18, delta)
			require.InDelta(t, float2, actual.Key19, delta)
			require.Equal(t, string7, actual.Key20[key2].Key22)
		})
	}
}

func TestDecodeAppManifest(t *testing.T) {
	var actual struct{ AppState *appState }
	require.NoError(t, vdf.Unmarshal([]byte(appManifest), &actual))

	require.Equal(t, &appState{
		AppID:      10,
		Name:       "Counter-Strike",
		StateFlags: 4,
		InstallDir: `Half-Life\cstrike`,
		Quote:      "say \"hi\"\n",
		WinPath:    `C:\Program Files`,
		Unquoted:   "bare_value",
		Platform:   "win",
		InstalledDepots: map[uint32]depot{
			1: {Manifest: 3101776004287380455, Size: 12},
			2: {Manifest: 18446744073709551615, Size: 34},
		},
		UserConfig: map[string]any{"language": "english"},
	}, actual.AppState)
}

func TestDecodeAny(t *testing.T) {
	var actual any
	require.NoError(t, vdf.Unmarshal([]byte(`{"a" {"b" {"c" "d"}} "e" "f"}`), &actual))
	expected := map[string]any{"a": map[string]any{"b": map[string]any{"c": "d"}}, "e": "f"}
	require.Equal(t, expected, actual)
}

func TestDecodeDuplicateObjectsMerge(t *testing.T) {
	var actual map[string]map[string]string
	require.NoError(t, vdf.Unmarshal([]byte(`"a" { "x" "1" } "a" { "y" "2" }`), &actual))
	expected := map[string]map[string]string{"a": {"x": "1", "y": "2"}}
	require.Equal(t, expected, actual)
}

func TestDecodeEmbeddedAndPointers(t *testing.T) {
	type Inner struct {
		A string
		B string
	}
	type Outer struct {
		*Inner
		B    string // shadows Inner.B
		Skip string `vdf:"-"`
	}

	var actual **Outer
	require.NoError(t, vdf.Unmarshal([]byte(`"A" "a" "B" "b" "Skip" "x"`), &actual))
	require.NotNil(t, actual)
	require.Equal(t, &Outer{Inner: &Inner{A: "a"}, B: "b"}, *actual)
}

func TestDecodeTextUnmarshaler(t *testing.T) {
	var actual struct{ Addr netip.Addr }
	require.NoError(t, vdf.Unmarshal([]byte(`"Addr" "127.0.0.1"`), &actual))
	require.Equal(t, netip.MustParseAddr("127.0.0.1"), actual.Addr)
}

func TestDecodeStream(t *testing.T) {
	var (
		d      = vdf.NewDecoder(strings.NewReader(`{ "a" "1" } { "a" "2" }`))
		actual []string
	)
	for {
		var v struct{ A string }
		err := d.Decode(&v)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		actual = append(actual, v.A)
	}

	require.Equal(t, []string{"1", "2"}, actual)
}

func TestDecodeErrors(t *testing.T) {
	var (
		syntaxErr *vdf.SyntaxError
		typeErr   *vdf.UnmarshalTypeError
		invalid   *vdf.InvalidUnmarshalError
	)

	for _, tc := range []struct {
		name string
		in   string
		v    any
		as   any
	}{
		{"int overflow", `"N" "300"`, &struct{ N int8 }{}, &typeErr},
		{"negative uint", `"N" "-1"`, &struct{ N uint }{}, &typeErr},
		{"bad bool", `"B" "yes"`, &struct{ B bool }{}, &typeErr},
		{"object into string", `"S" { }`, &struct{ S string }{}, &typeErr},
		{"string into struct", `"S" "x"`, &struct{ S struct{} }{}, &typeErr},
		{"bad map key", `"x" "1"`, &map[int]string{}, &typeErr},
		{"unterminated string", `"a" "b`, &map[string]string{}, &syntaxErr},
		{"missing close brace", `{ "a" "b"`, &map[string]string{}, &syntaxErr},
		{"stray close brace", `"a" "b" }`, &map[string]string{}, &syntaxErr},
		{"missing value", `"a"`, &map[string]string{}, &syntaxErr},
		{"max depth", strings.Repeat(`"a" {`, 20000), &map[string]any{}, &syntaxErr},
		{"non-pointer", `"a" "b"`, map[string]string{}, &invalid},
		{"nil pointer", `"a" "b"`, (*map[string]string)(nil), &invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorAs(t, vdf.Unmarshal([]byte(tc.in), tc.v), tc.as)
		})
	}

	t.Run("empty input", func(t *testing.T) {
		require.ErrorIs(t, vdf.Unmarshal([]byte("   // nothing\n"), &map[string]string{}), io.EOF)
	})
}
