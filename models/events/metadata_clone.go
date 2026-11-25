package events

import "reflect"

// cloneMetadata deep-copies a metadata map so that in-place writes through
// either copy (such as WithEventError setting metadata.error, or a producer
// appending to a []string value) never reach the other. Every reference kind
// is copied: maps, slices, arrays, pointers and interfaces, and the exported
// fields of structs, recursively. Shared and cyclic pointers, maps and slices
// keep their shape in the copy. Unexported struct fields, channels and funcs
// are copied by value, since reflection cannot set or meaningfully clone them.
func cloneMetadata(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	c := cloner{seen: map[cloneKey]reflect.Value{}}
	return c.clone(reflect.ValueOf(m)).Interface().(map[string]interface{})
}

// cloneKey identifies an already-copied pointer, map or slice by its address
// and type, so aliases and cycles map to the same copy.
type cloneKey struct {
	ptr uintptr
	typ reflect.Type
	len int
}

type cloner struct {
	seen map[cloneKey]reflect.Value
}

func (c cloner) clone(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		key := cloneKey{v.Pointer(), v.Type(), 0}
		if done, ok := c.seen[key]; ok {
			return done
		}
		out := reflect.New(v.Type().Elem())
		c.seen[key] = out
		out.Elem().Set(c.clone(v.Elem()))
		return out
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(c.clone(v.Elem()))
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		key := cloneKey{v.Pointer(), v.Type(), 0}
		if done, ok := c.seen[key]; ok {
			return done
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		c.seen[key] = out
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(c.clone(iter.Key()), c.clone(iter.Value()))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		key := cloneKey{v.Pointer(), v.Type(), v.Len()}
		if done, ok := c.seen[key]; ok {
			return done
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		c.seen[key] = out
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(c.clone(v.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(c.clone(v.Index(i)))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v) // carries unexported fields by value
		for i := 0; i < v.NumField(); i++ {
			if f := out.Field(i); f.CanSet() {
				f.Set(c.clone(v.Field(i)))
			}
		}
		return out
	default:
		return v
	}
}
