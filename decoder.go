package vdf

import (
	"bytes"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// InvalidUnmarshalError describes an invalid argument passed to Decode or Unmarshal.
// The argument must be a non-nil pointer.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "vdf: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Pointer {
		return "vdf: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "vdf: Unmarshal(nil " + e.Type.String() + ")"
}

// UnmarshalTypeError describes a VDF value that could not be stored in a Go value.
type UnmarshalTypeError struct {
	// Value is "string" or "object".
	Value string
	Type  reflect.Type
	// Key is the dot-separated path to the offending key.
	Key string
	Err error
}

func (e *UnmarshalTypeError) Error() string {
	msg := fmt.Sprintf("vdf: cannot unmarshal %s into Go value of type %s at key %q", e.Value, e.Type, e.Key)
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *UnmarshalTypeError) Unwrap() error {
	return e.Err
}

// Decoder reads and decodes VDF values from an input stream.
type Decoder struct {
	s *scanner
}

// NewDecoder returns a new decoder that reads from r.
// The decoder buffers its input and may read past the end of a VDF document.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{s: newScanner(r)}
}

// Decode reads the next VDF document from its input and stores it in the value pointed to by v.
//
// A document is either an object wrapped in braces or a sequence of key/value
// pairs running to EOF, which is the form written by Steam. Decode returns io.EOF
// if there is no more input.
//
// Objects decode into structs, maps with string or integer keys, and empty
// interfaces (as map[string]any). Struct fields are matched by their `vdf` tag
// or name, preferring an exact match and falling back to a case-insensitive one.
// Unknown keys are ignored. Strings decode into strings, integers, floats,
// bools, empty interfaces and implementations of encoding.TextUnmarshaler.
// When a key repeats, the last value wins, except that repeated objects
// decoded into the same struct or map field are merged.
func (d *Decoder) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	obj, err := d.s.parseDocument()
	if err != nil {
		return err
	}

	return decodeObject(obj, rv, "")
}

// Unmarshal parses the VDF-encoded data and stores the result in the value pointed to by v.
func Unmarshal(b []byte, v any) error {
	return NewDecoder(bytes.NewReader(b)).Decode(v)
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// indirect walks down pointers, allocating them as needed, until it reaches a
// non-pointer. If wantText is set and it encounters an encoding.TextUnmarshaler
// along the way, it returns that instead.
func indirect(v reflect.Value, wantText bool) (encoding.TextUnmarshaler, reflect.Value) {
	for {
		if wantText && v.Kind() != reflect.Pointer && v.CanAddr() && reflect.PointerTo(v.Type()).Implements(textUnmarshalerType) {
			return v.Addr().Interface().(encoding.TextUnmarshaler), v
		}

		if v.Kind() != reflect.Pointer {
			return nil, v
		}

		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
}

func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func decodePair(p pair, v reflect.Value, path string) error {
	if p.isObject {
		return decodeObject(p.object, v, path)
	}
	return decodeString(p.value, v, path)
}

func decodeString(s string, v reflect.Value, path string) error {
	u, v := indirect(v, true)
	if u != nil {
		if err := u.UnmarshalText([]byte(s)); err != nil {
			return &UnmarshalTypeError{Value: "string", Type: v.Type(), Key: path, Err: err}
		}
		return nil
	}

	var err error
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			return &UnmarshalTypeError{Value: "string", Type: v.Type(), Key: path}
		}
		v.Set(reflect.ValueOf(s))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		if n, err = strconv.ParseInt(s, 10, v.Type().Bits()); err == nil {
			v.SetInt(n)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var n uint64
		if n, err = strconv.ParseUint(s, 10, v.Type().Bits()); err == nil {
			v.SetUint(n)
		}
	case reflect.Float32, reflect.Float64:
		var n float64
		if n, err = strconv.ParseFloat(s, v.Type().Bits()); err == nil {
			v.SetFloat(n)
		}
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(s); err == nil {
			v.SetBool(b)
		}
	default:
		return &UnmarshalTypeError{Value: "string", Type: v.Type(), Key: path}
	}

	if err != nil {
		return &UnmarshalTypeError{Value: "string", Type: v.Type(), Key: path, Err: err}
	}

	return nil
}

func decodeObject(obj object, v reflect.Value, path string) error {
	_, v = indirect(v, false)

	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() != 0 {
			return &UnmarshalTypeError{Value: "object", Type: v.Type(), Key: path}
		}

		m, ok := reflect.TypeAssert[map[string]any](v)
		if !ok || m == nil {
			m = make(map[string]any, len(obj))
		}
		if err := decodeObject(obj, reflect.ValueOf(m), path); err != nil {
			return err
		}
		v.Set(reflect.ValueOf(m))
	case reflect.Map:
		t := v.Type()
		if v.IsNil() {
			v.Set(reflect.MakeMapWithSize(t, len(obj)))
		}

		for _, p := range obj {
			childPath := joinKey(path, p.key)

			kv, err := mapKey(t.Key(), p.key)
			if err != nil {
				return &UnmarshalTypeError{Value: "string", Type: t.Key(), Key: childPath, Err: err}
			}

			// Merge into an existing element rather than replacing it, so that
			// repeated object keys accumulate, as with struct fields.
			ev := reflect.New(t.Elem()).Elem()
			if existing := v.MapIndex(kv); existing.IsValid() && p.isObject {
				ev.Set(existing)
			}

			if err := decodePair(p, ev, childPath); err != nil {
				return err
			}

			v.SetMapIndex(kv, ev)
		}
	case reflect.Struct:
		fields := cachedFields(v.Type())

		for _, p := range obj {
			f, ok := fields.lookup(p.key)
			if !ok {
				continue
			}

			fv, ok := fieldByIndex(v, f.index, true)
			if !ok {
				continue
			}

			if err := decodePair(p, fv, joinKey(path, p.key)); err != nil {
				return err
			}
		}
	default:
		return &UnmarshalTypeError{Value: "object", Type: v.Type(), Key: path}
	}

	return nil
}

func mapKey(t reflect.Type, key string) (reflect.Value, error) {
	kv := reflect.New(t).Elem()

	switch t.Kind() {
	case reflect.String:
		kv.SetString(key)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(key, 10, t.Bits())
		if err != nil {
			return kv, err
		}
		kv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(key, 10, t.Bits())
		if err != nil {
			return kv, err
		}
		kv.SetUint(n)
	default:
		return kv, fmt.Errorf("unsupported map key type %s", t)
	}

	return kv, nil
}

// fieldByIndex is like reflect.Value.FieldByIndex, but when alloc is set it
// allocates nil embedded struct pointers, and otherwise reports false on them.
func fieldByIndex(v reflect.Value, index []int, alloc bool) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !alloc || !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v, true
}
