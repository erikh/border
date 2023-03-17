package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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

		literal, ok := r.LiteralValue[tag]
		if !ok && !optional {
			return fmt.Errorf("Could not find required literal value in config: %v", tag)
		}

		if ok {
			valueField := reflect.ValueOf(r.Value).Elem().Field(i)
			if err := typeAssert(valueField.Type(), literal, valueField); err != nil {
				return fmt.Errorf("Error while converting literal %q to type %q: %v", tag, valueField.Type().Name(), err)
			}
		}
	}

	return nil
}

// this is probably broken (and probably always will be) in some subtle ways.
// If you're having trouble with it, change it.
func typeAssert(typ reflect.Type, literal any, value reflect.Value) error {
	switch typ.Kind() {
	case reflect.Pointer, reflect.Interface:
		return typeAssert(typ.Elem(), literal, value)
	case reflect.Array, reflect.Slice:
		switch reflect.TypeOf(literal).Kind() {
		case reflect.Array, reflect.Slice:
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

		literalVal := reflect.ValueOf(literal)

		iter := literalVal.MapRange()

		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			if err := typeAssert(reflect.TypeOf(val.Interface()), val.Interface(), value.MapIndex(key.Elem())); err != nil {
				return err
			}
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

		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			strKey, ok := key.Interface().(string)
			if !ok {
				return fmt.Errorf("While translating map to struct, key was not string, was %T", key.Interface())
			}

			if err := typeAssert(reflect.TypeOf(val.Interface()), val.Interface(), value.FieldByName(strKey)); err != nil {
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
			value.Set(reflect.ValueOf(literal))
		}
	}

	return nil
}
