package influxmarshal

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	influx "github.com/influxdata/influxdb1-client"
)

// InfluxValuer is the interface for your type to return a tag or field value
type InfluxValuer interface {
	InfluxValue() (value interface{})
}

// Marshal returns an *influx.Point for v.
//
// Marshal traverses the first level of v. If an encountered value
// implements the InfluxValuer or fmt.Stringer interfaces and is not
// a nil pointer, Marshal will use the returned value to render the
// tag or field. Nil pointers are skipped.
//
// Otherwise, Marshal supports encoding integers, floats, strings and
// booleans.
//
// The encoding of each struct field can be customized by the format string
// stored under the "influx" key in the struct field's tag.
// The format string gives the name of the field, possibly followed by a
// comma-separated list of options. The name may be empty in order to
// specify options without overriding the default field name.
//
// The "omitzero" option specifies that the field should be omitted from the
// encoding if the field has an zero value as defined by reflect.IsZero() Note:
// implemented internally until it lands in tip.
// (ref: https://go-review.googlesource.com/c/go/+/171337/ )
//
// The "tag" option specifies that the field is a tag, and the value will be
// converted to a string, following InfluxDB specifications.
// (ref: https://docs.influxdata.com/influxdb/v1.7/concepts/key_concepts/#tag-value)
// If the "tag" option is not present the field will be treated as an
// InfluxDB field. (ref: https://docs.influxdata.com/influxdb/v1.7/concepts/key_concepts/#field-value)
//
// As a special case, if the field tag is "-", the field is always omitted.
// Note that a field with name "-" can still be generated using the tag "-,".
//
// Examples of struct field tags and their meanings:
//
//   // Value appears in InfluxDB as field with key "myName".
//   Value int `influx:"myName"`
//
//   // Value appears in InfluxDB as tag with key "myName" and stringified
//   // integer representation
//   Value int `influx:"myname,tag"`
//
//   // Value appears in InfluxDB as field with key "myName" but will be
//   // ommitted if it has a zero value as defined above.
//   Value int `influx:"myName,omitzero"`
//
//   // Value appears in InfluxDB as field with key "Value" (the default), but
//	 // will be ommitted if it has a zero value.
//   Value int `influx:",omitzero"`
//
//   // Value is ignored by this package.
//   Value int `influx:"-"`
//
//   // Value appears in InfluxDB with field key "-".
//   Value int `influx:"-,"`
//
// Anonymous struct fields will be marshaled with their package-local type name unless
// specified otherwise via tags.
//
// Pointer values encode as the value pointed to.
//
func Marshal(v interface{}, measurement string) (*influx.Point, error) {
	val := reflect.ValueOf(v)

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("value is nil")
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		// XXX: check interface here, first?
		return nil, fmt.Errorf("not a struct")
	}

	p := &influx.Point{
		Tags:        make(map[string]string),
		Fields:      make(map[string]interface{}),
		Time:        time.Now(),
		Measurement: measurement,
	}

	// TODO: Rename
	vType := val.Type()

	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		structField := vType.Field(i)

		if structField.PkgPath != "" {
			continue
		}
		opts := getOpts(structField)
		if opts == nil {
			continue
		}
		if f.Kind() == reflect.Ptr {
			if f.IsNil() {
				// XXX: Error here? Maybe if omitzero not specified?
				continue
			}
			f = f.Elem()
		}

		val := f.Interface()

		// find out if the type implements InfluxValuer or fmt.Stringer
		switch v := val.(type) {
		case InfluxValuer:
			val = v.InfluxValue()
		case fmt.Stringer:
			val = v.String()
		}

		// get new reflect.Value
		// XXX: or move ValueOf call to isZero and similarly for a influx type checking func
		vv := reflect.ValueOf(val)
		if opts.omitzero && isZero(vv) {
			continue
		}

		// Ensure this is a type Influx can handle
		switch vv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String, reflect.Bool:
			// we're good
		default:
			return nil, fmt.Errorf("Unsupported type for member %s", structField.Name)
		}

		if opts.tag {
			p.Tags[opts.name] = fmt.Sprint(val)
		} else {
			p.Fields[opts.name] = val
		}
	}
	return p, nil
}

type fieldOptions struct {
	name     string
	omitzero bool
	tag      bool
}

func getOpts(f reflect.StructField) *fieldOptions {
	o := &fieldOptions{
		name: f.Name,
	}
	val, ok := f.Tag.Lookup("influx")
	if val == "-" {
		return nil
	}
	if ok {
		opts := strings.Split(val, ",")
		if len(opts) > 0 {
			switch opts[0] {
			case "":
				// retain name
			default:
				// otherwise, use this name
				o.name = opts[0]
			}
			// process the rest of the options
			if len(opts) > 1 {
				for _, opt := range opts[1:] {
					switch opt {
					case "omitzero":
						o.omitzero = true
					case "tag":
						o.tag = true
					default:
						// TODO?: error reporting here?
					}
				}
			}
		}
	}
	return o
}

// Until https://go-review.googlesource.com/c/go/+/171337/ lands...
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(v.Float()) == 0
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return math.Float64bits(real(c)) == 0 && math.Float64bits(imag(c)) == 0
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		// This should never happens, but will act as a safeguard for
		// later, as a default value doesn't makes sense here.
		panic(fmt.Sprintf("isZero %v", v.Kind()))
	}
}
