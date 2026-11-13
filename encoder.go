package vdf

import (
	"bytes"
	"cmp"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// UnsupportedTypeError is returned by Marshal when attempting to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "vdf: unsupported type: " + e.Type.String()
}

// Encoder writes VDF documents to an output stream.
type Encoder struct {
	w io.Writer
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes the VDF encoding of v to the stream.
//
// v must be a struct or a map with string or integer keys, or a pointer to
// one. Its keys are written as top-level key/value pairs, the form used by
// Steam's own files, indented with tabs. Map keys are sorted, numerically for
// integer keys. Struct fields use the same names as Decode and honor the
// `vdf:"name,omitempty"` and `vdf:"-"` tag options; omitempty skips zero
// values and empty maps.
//
// Strings, integers, floats and implementations of encoding.TextMarshaler are
// written as strings, bools as "1" or "0", and structs and maps as nested
// objects. Nil pointers, interfaces and maps are omitted, since VDF has no null.
// Slices, arrays and other types are unsupported.
func (e *Encoder) Encode(v any) error {
	b, err := Marshal(v)
	if err != nil {
		return err
	}

	_, err = e.w.Write(b)
	return err
}

// Marshal returns the VDF encoding of v. See Encoder.Encode for details.
func Marshal(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, fmt.Errorf("vdf: cannot marshal nil %s", rv.Type())
		}
		rv = rv.Elem()
	}

	if !rv.IsValid() {
		return nil, fmt.Errorf("vdf: cannot marshal nil")
	}

	if k := rv.Kind(); k != reflect.Struct && k != reflect.Map {
		return nil, fmt.Errorf("vdf: cannot marshal %s at top level, must be a struct or map", rv.Type())
	}

	e := &encodeState{}
	if err := e.writeObject(rv, 0); err != nil {
		return nil, err
	}

	return e.Bytes(), nil
}

type encodeState struct {
	bytes.Buffer
}

func (e *encodeState) writeIndent(depth int) {
	for range depth {
		e.WriteByte('\t')
	}
}

var quoteReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)

func (e *encodeState) writeQuoted(s string) {
	e.WriteByte('"')
	_, _ = quoteReplacer.WriteString(e, s)
	e.WriteByte('"')
}

func (e *encodeState) writeObject(v reflect.Value, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("vdf: exceeded max depth of %d, possible cycle at %s", maxDepth, v.Type())
	}

	switch v.Kind() {
	case reflect.Map:
		type kv struct {
			key  string
			num  *intKey
			elem reflect.Value
		}

		kvs := make([]kv, 0, v.Len())
		for it := v.MapRange(); it.Next(); {
			k := it.Key()
			switch k.Kind() {
			case reflect.String:
				kvs = append(kvs, kv{key: k.String(), elem: it.Value()})
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				kvs = append(kvs, kv{key: strconv.FormatInt(k.Int(), 10), num: &intKey{neg: k.Int() < 0, abs: absInt(k.Int())}, elem: it.Value()})
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				kvs = append(kvs, kv{key: strconv.FormatUint(k.Uint(), 10), num: &intKey{abs: k.Uint()}, elem: it.Value()})
			default:
				return &UnsupportedTypeError{Type: v.Type()}
			}
		}

		slices.SortFunc(kvs, func(a, b kv) int {
			if a.num != nil && b.num != nil {
				return a.num.cmp(*b.num)
			}
			return cmp.Compare(a.key, b.key)
		})

		for _, kv := range kvs {
			if err := e.writePair(kv.key, kv.elem, depth); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for _, f := range cachedFields(v.Type()).list {
			fv, ok := fieldByIndex(v, f.index, false)
			if !ok {
				continue
			}

			if f.omitEmpty && isEmpty(fv) {
				continue
			}

			if err := e.writePair(f.name, fv, depth); err != nil {
				return err
			}
		}
	default:
		return &UnsupportedTypeError{Type: v.Type()}
	}

	return nil
}

var textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()

func (e *encodeState) writePair(key string, v reflect.Value, depth int) error {
	for {
		if v.Type().Implements(textMarshalerType) {
			if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
				return nil
			}

			b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
			if err != nil {
				return fmt.Errorf("vdf: error calling MarshalText for type %s: %w", v.Type(), err)
			}

			e.writeString(key, string(b), depth)
			return nil
		}

		if v.Kind() != reflect.Pointer && v.Kind() != reflect.Interface {
			break
		}

		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		e.writeString(key, v.String(), depth)
	case reflect.Bool:
		s := "0"
		if v.Bool() {
			s = "1"
		}
		e.writeString(key, s, depth)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.writeString(key, strconv.FormatInt(v.Int(), 10), depth)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		e.writeString(key, strconv.FormatUint(v.Uint(), 10), depth)
	case reflect.Float32, reflect.Float64:
		e.writeString(key, strconv.FormatFloat(v.Float(), 'f', -1, v.Type().Bits()), depth)
	case reflect.Map, reflect.Struct:
		if v.Kind() == reflect.Map && v.IsNil() {
			return nil
		}

		e.writeIndent(depth)
		e.writeQuoted(key)
		e.WriteByte('\n')
		e.writeIndent(depth)
		e.WriteString("{\n")
		if err := e.writeObject(v, depth+1); err != nil {
			return err
		}
		e.writeIndent(depth)
		e.WriteString("}\n")
	default:
		return &UnsupportedTypeError{Type: v.Type()}
	}

	return nil
}

func (e *encodeState) writeString(key, value string, depth int) {
	e.writeIndent(depth)
	e.writeQuoted(key)
	e.WriteString("\t\t")
	e.writeQuoted(value)
	e.WriteByte('\n')
}

func isEmpty(v reflect.Value) bool {
	if v.Kind() == reflect.Map {
		return v.Len() == 0
	}
	return v.IsZero()
}

// intKey is a sign-magnitude integer that orders any int64 or uint64 map key.
type intKey struct {
	neg bool
	abs uint64
}

func absInt(n int64) uint64 {
	if n < 0 {
		return uint64(-(n + 1)) + 1
	}
	return uint64(n)
}

func (a intKey) cmp(b intKey) int {
	switch {
	case a.neg && !b.neg:
		return -1
	case !a.neg && b.neg:
		return 1
	case a.neg:
		return cmp.Compare(b.abs, a.abs)
	default:
		return cmp.Compare(a.abs, b.abs)
	}
}
