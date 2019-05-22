# influxmarshal [![GoDoc](https://godoc.org/github.com/flowchartsman/influxmarshal?status.svg)](https://godoc.org/github.com/flowchartsman/influxmarshal)
--
    import "github.com/flowchartsman/influxmarshal"


## Usage

#### func  Marshal

```go
func Marshal(v interface{}, measurement string) (*influx.Point, error)
```
Marshal returns an *influx.Point for v.

Marshal traverses the first level of v. If an encountered value implements the
InfluxValuer or fmt.Stringer interfaces and is not a nil pointer, Marshal will
use the returned value to render the tag or field. Nil pointers are skipped.

Otherwise, Marshal supports encoding integers, floats, strings and booleans.

The encoding of each struct field can be customized by the format string stored
under the "influx" key in the struct field's tag. The format string gives the
name of the field, possibly followed by a comma-separated list of options. The
name may be empty in order to specify options without overriding the default
field name.

The "omitzero" option specifies that the field should be omitted from the
encoding if the field has an zero value as defined by reflect.IsZero() (note:
not currently implemented in tip, but coming soon ref:
https://go-review.googlesource.com/c/go/+/171337/ )

The "tag" option specifies that the field is a tag, and the value will be
converted to a string, following InfluxDB specifications. (ref:
https://docs.influxdata.com/influxdb/v1.7/concepts/key_concepts/#tag-value) If
the "tag" option is not present the field will be treated as an InfluxDB field.
(ref:
https://docs.influxdata.com/influxdb/v1.7/concepts/key_concepts/#field-value)

As a special case, if the field tag is "-", the field is always omitted. Note
that a field with name "-" can still be generated using the tag "-,".

Examples of struct field tags and their meanings:

      // Value appears in InfluxDB as field with key "myName".
      Value int `influx:"myName"`

      // Value appears in InfluxDB as tag with key "myName" and stringified
      // integer representation
      Value int `influx:"myname,tag"`

      // Value appears in InfluxDB as field with key "myName" but will be
      // ommitted if it has a zero value as defined above.
      Value int `influx:"myName,omitzero"`

      // Value appears in InfluxDB as field with key "Value" (the default), but
    	 // will be ommitted if it has a zero value.
      Value int `influx:",omitzero"`

      // Value is ignored by this package.
      Value int `influx:"-"`

      // Value appears in InfluxDB with field key "-".
      Value int `influx:"-,"`

Anonymous struct fields will be marshaled with their package-local type name
unless specified otherwise via tags.

Pointer values encode as the value pointed to.

#### type InfluxValuer

```go
type InfluxValuer interface {
	InfluxValue() (value interface{})
}
```

InfluxValuer is the interface for your type to return a tag or field value
