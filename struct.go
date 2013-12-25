// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bencode

import (
	"fmt"
	"reflect"
)

func isPOD(k reflect.Kind) bool {
	return k == reflect.Int64 || k == reflect.Uint64 || k == reflect.String
}

func isRawMessagePtr(typ reflect.Type) bool {
	return typ.Kind() == reflect.Ptr && typ.Elem().Name() == "RawMessage"
}

func unmarshalPOD(pod interface{}, val reflect.Value) error {
	podk := reflect.TypeOf(pod).Kind()
	if !isPOD(podk) {
		return fmt.Errorf("not a POD: %v", podk)
	}

	v := val.Elem()
	k := v.Type().Kind()
	if k == reflect.Int64 &&  podk == reflect.Uint64 {
		v.Set(reflect.ValueOf(int64(pod.(uint64))))
		return nil
	}

	if podk != k {
		return fmt.Errorf("mismatched type got: %v want: %v", k, podk)
	}

	v.Set(reflect.ValueOf(pod))
	return nil
}

func unmarshalList(l []*RawMessage, v reflect.Value) (err error) {
	typ := v.Type()
	if typ.Kind() != reflect.Slice {
		return fmt.Errorf("not a Slice: %v", typ.Kind())
	}

	elem := typ.Elem()
	for i, rm := range l {
		v.Set(reflect.Append(v, reflect.Zero(elem)))
		val := v.Index(i)
		switch {
		case isRawMessagePtr(elem):
			val.Set(reflect.ValueOf(rm))
		case rm.POD != nil:
			err = unmarshalPOD(rm.POD, val.Addr())
		case len(rm.L) > 0:
			err = unmarshalList(rm.L, val)
		case len(rm.D) > 0:
			val.Set(reflect.New(elem.Elem()))
			err = unmarshalDict(rm.D, val)
		}
		if err != nil {
			return
		}
	}
	return
}

func unmarshalDict(d []*KV, val reflect.Value) (err error) {
	kind := val.Type().Kind()
	if  kind != reflect.Ptr && kind != reflect.Map {
		return fmt.Errorf("not a Ptr/Map: %v", kind)
	}

	fields := map[string]reflect.Value{}
	if kind == reflect.Ptr {
		v := val.Elem()
		typ := v.Type()
		if typ.Kind() != reflect.Struct {
			return fmt.Errorf("not a Struct: %v", typ.Kind())
		}

		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if len(field.PkgPath) > 0 {
				continue // unexported
			}

			key := field.Name
			if len(field.Tag) > 0 {
				tag := field.Tag.Get("ben")
				if len(tag) > 0 {
					key = tag
				}
			}
			fields[key] = v.Field(i)
		}
	}

	for _, kv := range d {
		var field reflect.Value
		field, ok := fields[kv.K]
		if !ok {
			continue
		}

		rm := kv.V
		switch {
		case isRawMessagePtr(field.Type()):
			field.Set(reflect.ValueOf(rm))
		case rm.POD != nil:
			err = unmarshalPOD(rm.POD, field.Addr())
		case len(rm.L) > 0:
			err = unmarshalList(rm.L, field)
		case len(rm.D) > 0:
			if field.Type().Kind() == reflect.Map {
				field.Set(reflect.MakeMap(field.Type()))
			} else {
				field.Set(reflect.New(field.Type().Elem()))
			}
			err = unmarshalDict(rm.D, field)
		}
		if err != nil {
			return fmt.Errorf("field: %q: %v", kv.K, err)
		}
	}
	return nil
}

func marshalPOD(val reflect.Value) (rm *RawMessage) {
	if reflect.DeepEqual(val.Interface(), reflect.Zero(val.Type()).Interface()) {
		return nil
	}

	rm = &RawMessage{}
	rm.POD = val.Interface()
	return
}

func marshalList(val reflect.Value) (rm *RawMessage, err error) {
	if val.Len() == 0 {
		return nil, nil
	}

	rm = &RawMessage{}
	rm.L = []*RawMessage{}
	for i := 0; i < val.Len(); i++ {
		var m *RawMessage
		kind := val.Type().Elem().Kind()
		switch kind {
		case reflect.Int64, reflect.Uint64, reflect.String:
			m = marshalPOD(val.Index(i))
		case reflect.Slice:
			m, err = marshalList(val.Index(i))
		case reflect.Ptr:
			m, err = marshalDict(val.Index(i))
		}
		if err != nil {
			return nil, err
		}
		if m != nil {
			rm.L = append(rm.L, m)
		}
	}
	return
}

func marshalDict(val reflect.Value) (rm *RawMessage, err error) {
	v := val.Elem()
	typ := v.Type()
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("not a Struct: %v", typ.Kind())
	}

	if typ.Name() == "RawMessage" {
		return val.Interface().(*RawMessage), nil
	}

	rm = &RawMessage{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if len(field.PkgPath) > 0 {
			continue // unexported
		}

		key := field.Name
		if len(field.Tag) > 0 {
			tag := field.Tag.Get("ben")
			if len(tag) > 0 {
				key = tag
			}
		}

		kv := &KV{K: key}
		switch field.Type.Kind() {
		case reflect.Int64, reflect.Uint64, reflect.String:
			kv.V = marshalPOD(v.Field(i))
		case reflect.Slice:
			kv.V, err = marshalList(v.Field(i))
		case reflect.Ptr:
			kv.V, err = marshalDict(v.Field(i))
		}
		if err != nil {
			return nil, err
		}
		if kv.V != nil {
			rm.D = append(rm.D, kv)
		}
	}
	return rm, nil
}
