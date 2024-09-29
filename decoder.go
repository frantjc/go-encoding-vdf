package vdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"unicode"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

type state int

const (
	beforeObject state = iota
	enterObject
	beforeKeyOrEnd
	enterKey
	inKey
	exitKey
	beforeValue
	enterValue
	inValue
	exitValue
)

type Decoder struct {
	r io.Reader

	curState  state
	prevState state
}

// Decode reads the next VDF-encoded value from its input and stores it in the value pointed to by v.
//
//nolint:gocyclo
func (d *Decoder) Decode(v any) error {
	var (
		tagToIdxMap = map[string]int{}
		isMap       bool
		vRefType    reflect.Type
	)

	vRefVal, ok := v.(reflect.Value)
	if ok {
		vRefType = vRefVal.Type()
	} else {
		vRefVal = reflect.ValueOf(v)

		vRefType = vRefVal.Type()
		if vRefType.Kind() != reflect.Pointer {
			return fmt.Errorf("unexpected non-pointer type")
		}
	}

	for vRefType.Kind() == reflect.Pointer {
		vRefType = vRefType.Elem()
		vRefVal = vRefVal.Elem()
	}

	switch vRefType.Kind() {
	case reflect.Struct:
		for i := 0; i < vRefType.NumField(); i++ {
			var (
				field = vRefType.Field(i)
				tag   = field.Name
			)
			if t := vRefType.Field(i).Tag.Get("vdf"); t != "" {
				tag = t
			}

			tagToIdxMap[tag] = i
		}
	case reflect.Map:
		isMap = true
		// default:
		// 	return fmt.Errorf("invalid kind: %s", vRefType.Kind())
	}

	var (
		mem, key, val                []byte
		endOuter, endInner, recursed bool
	)
	for {
		if endOuter {
			break
		}

		p := make([]byte, 64)
		n, err := d.r.Read(p)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}

		var stateChngIdx int
		for i, b := range p[:n] {
			c := string(b)
			_ = c
			if endInner {
				d.r = io.MultiReader(bytes.NewReader(p[i:n]), d.r)
				endInner = false
				break
			}

			switch d.curState {
			case beforeObject:
				switch {
				case b == '{':
					d.curState = enterObject
				case unicode.IsSpace(rune(b)):
				default:
					return fmt.Errorf("unexpected symbol %c looking for beginning of object around: %x", b, p)
				}
			case enterObject:
				switch {
				case unicode.IsSpace(rune(b)):
					d.curState = beforeKeyOrEnd
				default:
					return fmt.Errorf("unexpected symbol %c after object start around: %s", b, p)
				}
			case beforeKeyOrEnd:
				switch {
				case b == '}':
					endOuter = true
					endInner = true
					d.curState = exitValue
				case b == '"':
					d.curState = enterKey
				case unicode.IsSpace(rune(b)):
				default:
					return fmt.Errorf("unexpected symbol %c looking for beginning of key around: %s", b, p)
				}
			case enterKey:
				switch {
				case b == '"':
					return fmt.Errorf("unexpected empty key around: %s", p)
				default:
					d.curState = inKey
				}
			case inKey:
				if b == '"' {
					key = mem
					mem = nil
					key = append(key, p[stateChngIdx:i]...)
					d.curState = exitKey
				}
			case exitKey:
				switch {
				case unicode.IsSpace(rune(b)):
					d.curState = beforeValue
				default:
					return fmt.Errorf("unexpected symbol %c after key %s around: %s", b, key, p)
				}
			case beforeValue:
				switch {
				case b == '{':
					d.prevState = d.curState
					d.curState = beforeKeyOrEnd
					if i < n-1 {
						writeBack := p[i+1 : n]
						d.r = io.MultiReader(bytes.NewReader(writeBack), d.r)
					}

					if j, ok := tagToIdxMap[string(key)]; ok {
						field := vRefVal.Field(j)

						switch field.Kind() {
						case reflect.Map:
							if field.IsNil() && field.CanSet() {
								field.Set(
									reflect.MakeMap(
										reflect.MapOf(
											field.Type().Key(),
											field.Type().Elem(),
										),
									),
								)
							}
						case reflect.Pointer:
							if field.IsNil() && field.CanSet() {
								field.Set(
									reflect.New(field.Type().Elem()),
								)
							}
						}

						if err = d.Decode(field); err != nil {
							return err
						}
					} else if isMap {
						if vRefVal.IsNil() && vRefVal.CanSet() {
							vRefVal.Set(
								reflect.MakeMap(
									reflect.MapOf(
										vRefVal.Type().Key(),
										vRefVal.Type().Elem(),
									),
								),
							)
						}

						field := reflect.New(vRefVal.Type().Elem())

						if err = d.Decode(field); err != nil {
							return err
						}

						vRefVal.SetMapIndex(reflect.ValueOf(string(key)), field.Elem())
					} else {
						if err = d.Decode(&map[string]any{}); err != nil {
							return err
						}
					}

					key = nil
					d.curState = beforeKeyOrEnd
					recursed = true
				case b == '"':
					d.curState = enterValue
				case unicode.IsSpace(rune(b)):
				default:
					return fmt.Errorf("unexpected symbol %c looking for beginning of value around: %s", b, p)
				}
			case enterValue:
				switch {
				case b == '"':
					val = []byte("")
					d.curState = exitValue
				default:
					d.curState = inValue
				}
			case inValue:
				if b == '"' {
					val = mem
					mem = nil
					val = append(val, p[stateChngIdx:i]...)
					d.curState = exitValue
				}
			case exitValue:
				switch {
				case unicode.IsSpace(rune(b)):
					d.curState = beforeKeyOrEnd
				default:
					return fmt.Errorf("unexpected symbol %c after value %s around: %s", b, val, p)
				}
			}

			if recursed {
				recursed = false
				break
			} else if d.prevState != d.curState {
				stateChngIdx = i
				d.prevState = d.curState
			}

			if key != nil && val != nil {
				if isMap {
					vRefVal.SetMapIndex(reflect.ValueOf(string(key)), reflect.ValueOf(string(val)))
				} else {
					if i, ok := tagToIdxMap[string(key)]; ok {
						if field := vRefVal.Field(i); field.CanSet() {
							switch field.Kind() {
							case reflect.String:
								field.SetString(string(val))
							case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
								pVal, err := strconv.Atoi(string(val))
								if err != nil {
									return err
								}

								field.SetInt(int64(pVal))
							case reflect.Bool:
								pVal, err := strconv.ParseBool(string(val))
								if err != nil {
									return err
								}

								field.SetBool(pVal)
							case reflect.Float32, reflect.Float64:
								bitSize := 64
								if field.Kind() == reflect.Float32 {
									bitSize = 32
								}

								pVal, err := strconv.ParseFloat(string(val), bitSize)
								if err != nil {
									return err
								}

								field.SetFloat(pVal)
							default:
								return fmt.Errorf("set unknown kind %s", field.Kind())
							}
						}
					}
				}

				key = nil
				val = nil
			}
		}

		switch d.curState {
		case inKey, inValue:
			mem = append(mem, p[stateChngIdx:n]...)
		}
	}

	return nil
}

// Unmarshal parses the VDF-encoded data and stores the result in the value pointed to by v.
func Unmarshal(b []byte, v any) error {
	return (&Decoder{r: bytes.NewReader(b)}).Decode(v)
}
