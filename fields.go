package vdf

import (
	"reflect"
	"slices"
	"strings"
	"sync"
)

type field struct {
	name      string
	index     []int
	omitEmpty bool
}

type structFields struct {
	// list holds the visible fields in declaration order.
	list   []field
	byName map[string]int
	byFold map[string]int
}

func (f *structFields) lookup(key string) (field, bool) {
	if i, ok := f.byName[key]; ok {
		return f.list[i], true
	}
	if i, ok := f.byFold[strings.ToLower(key)]; ok {
		return f.list[i], true
	}
	return field{}, false
}

var fieldCache sync.Map // map[reflect.Type]*structFields

// cachedFields returns the fields of struct type t that participate in
// encoding and decoding. A field is named by its `vdf` tag if present and by
// its Go name otherwise. A tag of "-" omits the field. Fields of embedded
// structs without a tag are promoted; a shallower field shadows a deeper one.
func cachedFields(t reflect.Type) *structFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(*structFields)
	}

	var all []field
	collectFields(t, nil, map[reflect.Type]bool{t: true}, &all)

	// Pick the shallowest field for each name; ties go to the first declared.
	winner := map[string]int{}
	for i, f := range all {
		if j, ok := winner[f.name]; !ok || len(f.index) < len(all[j].index) {
			winner[f.name] = i
		}
	}

	sf := &structFields{byName: map[string]int{}, byFold: map[string]int{}}
	for i, f := range all {
		if winner[f.name] != i {
			continue
		}
		sf.byName[f.name] = len(sf.list)
		if _, ok := sf.byFold[strings.ToLower(f.name)]; !ok {
			sf.byFold[strings.ToLower(f.name)] = len(sf.list)
		}
		sf.list = append(sf.list, f)
	}

	f, _ := fieldCache.LoadOrStore(t, sf)
	return f.(*structFields)
}

func collectFields(t reflect.Type, index []int, visited map[reflect.Type]bool, out *[]field) {
	for i := range t.NumField() {
		sf := t.Field(i)

		tag := sf.Tag.Get("vdf")
		if tag == "-" {
			continue
		}

		name, opts, _ := strings.Cut(tag, ",")
		idx := append(slices.Clone(index), i)

		if sf.Anonymous && name == "" {
			ft := sf.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if !visited[ft] {
					visited[ft] = true
					collectFields(ft, idx, visited, out)
					delete(visited, ft)
				}
				continue
			}
		}

		if !sf.IsExported() {
			continue
		}

		if name == "" {
			name = sf.Name
		}

		*out = append(*out, field{
			name:      name,
			index:     idx,
			omitEmpty: slices.Contains(strings.Split(opts, ","), "omitempty"),
		})
	}
}
