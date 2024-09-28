package vdf_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	vdf "github.com/frantjc/go-encoding-vdf"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

const (
	letters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lenLetters   = len(letters)
	maxInt       = 16
	floatEpsilon = 0.0001
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

func floatEqual[T float32 | float64](a, b T) bool {
	return math.Abs(float64(a)-float64(b)) <= floatEpsilon
}

func TestDecode(t *testing.T) {
	for i := 0; i < rnd.Intn(maxInt); i++ {
		var (
			string1 = randString()
			string2 = randString()
			string3 = randString()
			key1    = randString()
			string4 = randString()
			string5 = randString()
			string6 = randString()
			int1    = rnd.Int()
			int2    = rnd.Int()
			int3    = rnd.Int()
			int4    = rnd.Int()
			int5    = rnd.Int()
			float1  = rnd.Float32()
			float2  = rnd.Float64()
			key2    = randString()
			string7 = randString()
			data    = []byte(fmt.Sprintf(`{
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
	}`, string1, string2, string3, key1, string4, string5, string6, int1, int2, int3, int4, int5, float1, float2, key2, string7))
			obj = struct {
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
			}{}
		)

		//nolint:gocritic
		if err := vdf.NewDecoder(bytes.NewReader(data)).Decode(&obj); err != nil {
			t.Fatal(err)
		} else if obj.Key1 != string1 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key1, string1)
		} else if obj.Key2 != string2 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key2, string2)
		} else if obj.Key3.Key4 != string3 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key3.Key4, string3)
		} else if obj.Key5[key1] != string4 {
			t.Fatalf("unexpected value %s looking for %s in key %s", obj.Key5[key1], string4, key1)
		} else if obj.Key9 == nil {
			t.Fatalf("unexpected nil struct pointer")
		} else if obj.Key9.Key10 != string6 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key9.Key10, string6)
		} else if obj.Key11 != "" {
			t.Fatalf("unexpected value %s looking for empty string", obj.Key11)
		} else if obj.Key12 != int1 {
			t.Fatalf("unexpected value %d looking for %d", obj.Key12, int1)
		} else if obj.Key13 != int8(int2) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key13, int2)
		} else if obj.Key14 != int16(int3) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key14, int3)
		} else if obj.Key15 != int32(int4) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key15, int4)
		} else if obj.Key16 != int64(int5) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key16, int5)
		} else if !obj.Key17 {
			t.Fatalf("unexpected value %t looking for %t", obj.Key17, true)
		} else if !floatEqual(obj.Key18, float1) {
			t.Fatalf("unexpected value %f looking for %f", obj.Key18, float1)
		} else if !floatEqual(obj.Key19, float2) {
			t.Fatalf("unexpected value %f looking for %f", obj.Key19, float2)
		} else if obj.Key20[key2].Key22 != string7 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key20[key2].Key22, string7)
		}
	}
}
