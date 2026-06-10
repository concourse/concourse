package binder

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Unmarshaler is the structural equivalent of go-flags' flags.Unmarshaler,
// declared locally so the custom flag types (flag.URL, flag.PrivateKey,
// wrappa.LimitedRoute, ...) keep working without importing go-flags here.
type Unmarshaler interface {
	UnmarshalFlag(value string) error
}

func convertUnmarshal(val string, retval reflect.Value) (bool, error) {
	if retval.Type().NumMethod() > 0 && retval.CanInterface() {
		if unmarshaler, ok := retval.Interface().(Unmarshaler); ok {
			if retval.IsNil() {
				retval.Set(reflect.New(retval.Type().Elem()))
				unmarshaler = retval.Interface().(Unmarshaler)
			}
			return true, unmarshaler.UnmarshalFlag(val)
		}
	}

	if retval.Type().Kind() != reflect.Ptr && retval.CanAddr() {
		return convertUnmarshal(val, retval.Addr())
	}

	if retval.Type().Kind() == reflect.Interface && !retval.IsNil() {
		return convertUnmarshal(val, retval.Elem())
	}

	return false, nil
}

// convert reproduces go-flags' string-to-value conversion: Unmarshaler
// first, then time.Duration, then the basic kinds. Slices append one
// element per call; maps insert one `key:value` pair per call.
func convert(val string, retval reflect.Value) error {
	if ok, err := convertUnmarshal(val, retval); ok {
		return err
	}

	tp := retval.Type()

	if tp == reflect.TypeOf((*time.Duration)(nil)).Elem() {
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		retval.SetInt(int64(parsed))
		return nil
	}

	switch tp.Kind() {
	case reflect.String:
		retval.SetString(val)
	case reflect.Bool:
		// an empty value (e.g. CONCOURSE_X= in the environment) turns a
		// boolean flag on, as in go-flags
		if val == "" {
			retval.SetBool(true)
		} else {
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			retval.SetBool(b)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(val, 0, tp.Bits())
		if err != nil {
			return err
		}
		retval.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(val, 0, tp.Bits())
		if err != nil {
			return err
		}
		retval.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(val, tp.Bits())
		if err != nil {
			return err
		}
		retval.SetFloat(parsed)
	case reflect.Slice:
		elemval := reflect.New(tp.Elem())
		if err := convert(val, elemval.Elem()); err != nil {
			return err
		}
		retval.Set(reflect.Append(retval, elemval.Elem()))
	case reflect.Map:
		parts := strings.SplitN(val, ":", 2)

		key := parts[0]
		var value string
		if len(parts) == 2 {
			value = parts[1]
		}

		keyval := reflect.New(tp.Key())
		if err := convert(key, keyval); err != nil {
			return err
		}

		valueval := reflect.New(tp.Elem())
		if err := convert(value, valueval); err != nil {
			return err
		}

		if retval.IsNil() {
			retval.Set(reflect.MakeMap(tp))
		}

		retval.SetMapIndex(keyval.Elem(), valueval.Elem())
	case reflect.Ptr:
		if retval.IsNil() {
			retval.Set(reflect.New(tp.Elem()))
		}
		return convert(val, retval.Elem())
	case reflect.Interface:
		if !retval.IsNil() {
			return convert(val, retval.Elem())
		}
	}

	return nil
}

// isBoolType reports whether the field behaves as a boolean flag (taking
// no argument on the command line), unwrapping pointers and slices like
// go-flags' Option.isBool.
func isBoolType(tp reflect.Type) bool {
	for {
		switch tp.Kind() {
		case reflect.Slice, reflect.Ptr:
			tp = tp.Elem()
		case reflect.Bool:
			return true
		default:
			return false
		}
	}
}
