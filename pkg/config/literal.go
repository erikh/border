package config

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"
)

const RecordTag = "record"

func (r *Record) parseLiteral() error {
	typ := reflect.TypeOf(r.Value).Elem()

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("Value (%T) is not struct, is %v; should be struct", r.Value, typ)
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		tag, ok := field.Tag.Lookup(RecordTag)
		if !ok {
			return fmt.Errorf("Struct element was not record-tagged, please report this bug: %v", field.Name)
		}

		parts := strings.Split(tag, ",")
		var optional bool

		for i := 1; i < len(parts); i++ {
			switch parts[i] {
			case "optional":
				optional = true
			}
		}

		tag = parts[0]

		literal, ok := r.LiteralValue[tag]
		if !ok && !optional {
			return fmt.Errorf("Could not find required literal value in config: %v", tag)
		}

		if ok {
			valueField := reflect.ValueOf(r.Value).Elem().Field(i)
			if err := typeAssert(valueField.Type(), literal, valueField); err != nil {
				return fmt.Errorf("Error while converting literal %q: %v", tag, err)
			}
		}
	}

	return nil
}

// this is probably broken (and probably always will be) in some subtle ways.
// If you're having trouble with it, change it.
func typeAssert(typ reflect.Type, literal any, value reflect.Value) error {
	switch typ.Kind() {
	case reflect.Pointer:
		return typeAssert(typ.Elem(), literal, value)
	case reflect.Interface:
		return typeAssert(reflect.TypeOf(value), literal, value)
	case reflect.Array, reflect.Slice:
		switch reflect.TypeOf(literal).Kind() {
		case reflect.Array, reflect.Slice:
			switch fmt.Sprintf("%T", value.Interface()) { // going to hell for this
			case "[]net.IP":
				ips := []net.IP{}

				switch lit := literal.(type) {
				case []string:
					for _, str := range lit {
						ips = append(ips, net.ParseIP(str))
					}

					return typeAssert(value.Type(), ips, value)
				}
			}
		case reflect.String:
			switch fmt.Sprintf("%T", value.Interface()) { // going to hell for this
			case "net.IP": // net.IP is a []byte under the hood, need to account for the fall-through here
				switch lit := literal.(type) {
				case string:
					ip := net.ParseIP(lit)
					return typeAssert(value.Type(), ip, value)
				default:
					return fmt.Errorf("literal is %T, value is net.IP; data mismatch", literal)
				}
			}
		default:
			return fmt.Errorf("literal is %T, value is array or slice; data mismatch", literal)
		}

		literalVal := reflect.ValueOf(literal)

		if typ.Kind() == reflect.Slice {
			value.Grow(literalVal.Len() - value.Len())
			value.SetLen(literalVal.Len())
		}

		for i := 0; i < literalVal.Len(); i++ {
			idx := value.Index(i)
			if err := typeAssert(reflect.TypeOf(idx.Interface()), literalVal.Index(i).Interface(), idx); err != nil {
				return err
			}
		}
	case reflect.Map:
		if reflect.TypeOf(literal).Kind() != reflect.Map {
			return fmt.Errorf("literal is %T, value is map; data mismatch", literal)
		}

		if value.IsZero() {
			value.Set(reflect.MakeMap(value.Type()))
		}

		literalVal := reflect.ValueOf(literal)

		iter := literalVal.MapRange()

		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			value.SetMapIndex(key, val.Elem())
		}
	case reflect.Struct:
		var (
			literalVal reflect.Value
			iter       *reflect.MapIter
		)

		switch literal.(type) {
		case map[string]any:
			literalVal = reflect.ValueOf(literal)
			iter = literalVal.MapRange()
		default:
			return fmt.Errorf("literal was expected to be map[string]any but is %T", literal)
		}

		valueTyp := reflect.TypeOf(value.Interface())
		if valueTyp.Kind() == reflect.Pointer {
			valueTyp = valueTyp.Elem()
			if value.IsNil() {
				tmp := reflect.New(valueTyp)
				value.Set(tmp)
			}
		}

		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			strKey, ok := key.Interface().(string)
			if !ok {
				return fmt.Errorf("While translating map to struct, key was not string, was %T", key.Interface())
			}

			var (
				rec        string
				valueField reflect.Value
			)

			for i := 0; i < valueTyp.NumField(); i++ {
				field := valueTyp.Field(i)
				rec, ok = field.Tag.Lookup(RecordTag)
				if ok && rec == strKey {
					if value.Type().Kind() == reflect.Pointer {
						valueField = value.Elem().Field(i)
					} else {
						valueField = value.Field(i)
					}
					break
				}
			}

			if !ok {
				return fmt.Errorf("Inner struct %T for %q field did not have record tag", literal, strKey)
			}

			if err := typeAssert(reflect.TypeOf(val.Interface()), val.Interface(), valueField); err != nil {
				return err
			}
		}
	case reflect.Chan:
		return errors.New("Records with channels are not supported. Change the type or fix the code.")
	case reflect.Func:
		return errors.New("Records with functions are not supported. Change the type or fix the code.")
	case reflect.UnsafePointer:
		return errors.New("Records with unsafe pointers are not supported. Change the type or fix the code.")
	default:
		if value.CanSet() {
			switch value.Kind() {
			case reflect.Int:
				switch typ := literal.(type) {
				case float64:
					value.Set(reflect.ValueOf(int(typ)))
				default:
					value.Set(reflect.ValueOf(int(typ.(int))))
				}
			case reflect.Int8:
				value.Set(reflect.ValueOf(literal.(int8)))
			case reflect.Int16:
				value.Set(reflect.ValueOf(literal.(int16)))
			case reflect.Int32:
				value.Set(reflect.ValueOf(literal.(int32)))
			case reflect.Int64:
				switch typ := literal.(type) {
				case string:
					switch fmt.Sprintf("%T", value.Interface()) { // going to hell for this
					case "time.Duration":
						duration, err := time.ParseDuration(typ)
						if err != nil {
							return err
						}

						value.Set(reflect.ValueOf(duration))
					}
				case time.Duration:
					value.Set(reflect.ValueOf(typ))
				default:
					value.Set(reflect.ValueOf(literal.(int64)))
				}
			case reflect.Uint:
				value.Set(reflect.ValueOf(literal.(uint)))
			case reflect.Uint8:
				value.Set(reflect.ValueOf(literal.(uint8)))
			case reflect.Uint16:
				value.Set(reflect.ValueOf(literal.(uint16)))
			case reflect.Uint32:
				value.Set(reflect.ValueOf(literal.(uint32)))
			case reflect.Uint64:
				value.Set(reflect.ValueOf(literal.(uint64)))
			case reflect.Uintptr:
				value.Set(reflect.ValueOf(literal.(uintptr)))
			case reflect.Float32:
				value.Set(reflect.ValueOf(literal.(float32)))
			case reflect.Float64:
				value.Set(reflect.ValueOf(literal.(float64)))
			default:
				value.Set(reflect.ValueOf(literal))
			}
		}
	}

	return nil
}
