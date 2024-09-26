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
	letters    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lenLetters = len(letters)
	max        = 16
)

func randString() string {
	var (
		n = rnd.Intn(max) + 1
		b = make([]byte, n)
	)
	for i := range b {
		b[i] = letters[rnd.Intn(lenLetters)]
	}

	return string(b)
}

func TestDecode(t *testing.T) {
	for i := 0; i < rnd.Intn(max); i++ {
		var (
			val1  = randString()
			val2  = randString()
			val3  = randString()
			key1  = randString()
			val4  = randString()
			val5  = randString()
			val6  = randString()
			val7  = rnd.Int()
			val8  = rnd.Int()
			val9  = rnd.Int()
			val10 = rnd.Int()
			val11 = rnd.Int()
			val12 = rnd.Float32()
			val13 = rnd.Float64()
			data  = []byte(fmt.Sprintf(`{
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
	}`, val1, val2, val3, key1, val4, val5, val6, val7, val8, val9, val10, val11, val12, val13))
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
			}{}
		)

		//nolint:gocritic
		if err := vdf.NewDecoder(bytes.NewReader(data)).Decode(&obj); err != nil {
			t.Fatal(err)
		} else if obj.Key1 != val1 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key1, val1)
		} else if obj.Key2 != val2 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key2, val2)
		} else if obj.Key3.Key4 != val3 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key3.Key4, val3)
		} else if obj.Key5[key1] != val4 {
			t.Fatalf("unexpected value %s looking for %s in key %s", obj.Key5[key1], val4, key1)
		} else if obj.Key9 == nil {
			t.Fatalf("unexpected nil struct pointer")
		} else if obj.Key9.Key10 != val6 {
			t.Fatalf("unexpected value %s looking for %s", obj.Key9.Key10, val6)
		} else if obj.Key11 != "" {
			t.Fatalf("unexpected value %s looking for empty string", obj.Key11)
		} else if obj.Key12 != val7 {
			t.Fatalf("unexpected value %d looking for %d", obj.Key12, val7)
		} else if obj.Key13 != int8(val8) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key13, val8)
		} else if obj.Key14 != int16(val9) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key14, val9)
		} else if obj.Key15 != int32(val10) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key15, val10)
		} else if obj.Key16 != int64(val11) {
			t.Fatalf("unexpected value %d looking for %d", obj.Key16, val11)
		} else if !obj.Key17 {
			t.Fatalf("unexpected value %t looking for %t", obj.Key17, true)
		} else if !floatEqual(obj.Key18, val12) {
			t.Fatalf("unexpected value %f looking for %f", obj.Key18, val12)
		} else if !floatEqual(obj.Key19, val13) {
			t.Fatalf("unexpected value %f looking for %f", obj.Key19, val13)
		}
	}
}

func floatEqual[T float32 | float64](a, b T) bool {
	return math.Abs(float64(a)-float64(b)) <= 0.0001
}
