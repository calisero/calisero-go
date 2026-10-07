package calisero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// maxSafeInteger is the largest integer a JSON number holds exactly, 2^53 - 1.
const maxSafeInteger = 1<<53 - 1

// fieldError is a missing or malformed field of a decoded JSON value.
type fieldError struct {
	message string
}

func (e *fieldError) Error() string {
	return e.message
}

// decodeJSON decodes a JSON document: objects as map[string]any, arrays as []any,
// numbers as float64.
func decodeJSON(data []byte) (any, error) {
	var value any

	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// unmarshalResource is the UnmarshalJSON of a resource: the JSON null leaves the
// target as it is, as for the types encoding/json decodes itself.
func unmarshalResource[T any](data []byte, target *T, context string, read func(any, string) (T, error)) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	value, err := decodeJSON(data)
	if err != nil {
		return err
	}

	resource, err := read(value, context)
	if err != nil {
		return err
	}

	*target = resource

	return nil
}

// reader reads the fields of a decoded JSON object, in the way the API documents
// them: a required field that is missing or of the wrong type is an error naming
// it, an optional one reads as its zero value (nil for a pointer). Reads after the
// first error return zero values, and the first error is kept.
type reader struct {
	data    map[string]any
	context string
	err     error
}

// newReader reads value, which must be an object; context names it in errors.
func newReader(value any, context string) *reader {
	data, ok := value.(map[string]any)

	if !ok {
		message := fmt.Sprintf("invalid %s: expected an object, got %s", context, describe(value, true))

		return &reader{context: context, err: &fieldError{message}}
	}

	return &reader{data: data, context: context}
}

// fail keeps err, unless an error was kept already.
func (r *reader) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// invalid keeps the error of a field that is not what it must be.
func (r *reader) invalid(key, expected string) {
	value, present := r.data[key]

	r.fail(&fieldError{fmt.Sprintf("invalid %s: %q must be %s, got %s", r.context, key, expected, describe(value, present))})
}

// str is a required string.
func (r *reader) str(key string) string {
	if r.err != nil {
		return ""
	}

	value, ok := r.data[key].(string)
	if !ok {
		r.invalid(key, "a string")
	}

	return value
}

// nonEmpty is a required string that may not be empty.
func (r *reader) nonEmpty(key string) string {
	if r.err != nil {
		return ""
	}

	value, ok := r.data[key].(string)
	if !ok || value == "" {
		r.invalid(key, "a non-empty string")
	}

	return value
}

// optStr is an optional string: empty when the field is absent, null or not a string.
func (r *reader) optStr(key string) string {
	if r.err != nil {
		return ""
	}

	value, _ := r.data[key].(string)

	return value
}

// integer is a required integer.
func (r *reader) integer(key string) int {
	if r.err != nil {
		return 0
	}

	value, ok := toInt(r.data[key])
	if !ok {
		r.invalid(key, "an integer")
	}

	return value
}

// optInt is an optional integer: nil when the field is absent, null or not an integer.
func (r *reader) optInt(key string) *int {
	if r.err != nil {
		return nil
	}

	value, ok := toInt(r.data[key])
	if !ok {
		return nil
	}

	return &value
}

// number is a required finite number, or a string holding one: the API may send
// an amount as a string.
func (r *reader) number(key string) float64 {
	if r.err != nil {
		return 0
	}

	value, ok := toNumber(r.data[key])
	if !ok {
		r.invalid(key, "a number")
	}

	return value
}

// optNumber is an optional number, or a string holding one: nil when the field is
// absent, null or not a number.
func (r *reader) optNumber(key string) *float64 {
	if r.err != nil {
		return nil
	}

	value, ok := toNumber(r.data[key])
	if !ok {
		return nil
	}

	return &value
}

// boolean is a required boolean.
func (r *reader) boolean(key string) bool {
	if r.err != nil {
		return false
	}

	value, ok := r.data[key].(bool)
	if !ok {
		r.invalid(key, "a boolean")
	}

	return value
}

// object is a required object.
func (r *reader) object(key string) map[string]any {
	if r.err != nil {
		return nil
	}

	value, ok := r.data[key].(map[string]any)
	if !ok {
		r.invalid(key, "an object")
	}

	return value
}

// array is a required array.
func (r *reader) array(key string) []any {
	if r.err != nil {
		return nil
	}

	value, ok := r.data[key].([]any)
	if !ok {
		r.invalid(key, "an array")
	}

	return value
}

// optArray is an optional array: empty when the field is absent, null or not an array.
func (r *reader) optArray(key string) []any {
	if r.err != nil {
		return nil
	}

	value, _ := r.data[key].([]any)

	return value
}

// timestamp is a required date-time.
func (r *reader) timestamp(key string) time.Time {
	if r.err != nil {
		return time.Time{}
	}

	text, _ := r.data[key].(string)

	value, ok := parseTime(text)
	if !ok {
		r.invalid(key, "a date-time")
	}

	return value
}

// optTimestamp is an optional date-time: nil when the field is absent, null, or
// not a date-time.
func (r *reader) optTimestamp(key string) *time.Time {
	if r.err != nil {
		return nil
	}

	text, _ := r.data[key].(string)

	value, ok := parseTime(text)
	if !ok {
		return nil
	}

	return &value
}

// readList reads every item of a list with read, naming each one by its index in
// errors; the first error is kept by r.
func readList[T any](r *reader, items []any, context string, read func(any, string) (T, error)) []T {
	list := make([]T, 0, len(items))

	for index, item := range items {
		value, err := read(item, context+"["+strconv.Itoa(index)+"]")
		if err != nil {
			r.fail(err)

			return nil
		}

		list = append(list, value)
	}

	return list
}

// toInt is a decoded JSON number as an int, when it is an integer held exactly.
func toInt(value any) (int, bool) {
	number, ok := value.(float64)

	if !ok || number != math.Trunc(number) || math.Abs(number) > maxSafeInteger ||
		number > math.MaxInt || number < math.MinInt {
		return 0, false
	}

	return int(number), true
}

// toNumber is a decoded JSON number, or a string holding a finite number.
func toNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return 0, false
		}

		return number, true
	default:
		return 0, false
	}
}

// parseTime reads a date-time as the API writes it: ISO 8601 with a time zone,
// e.g. 2025-02-06T10:18:43.000000Z, or YYYY-MM-DD HH:MM:SS in UTC.
func parseTime(value string) (time.Time, bool) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}

	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsed, true
	}

	return time.Time{}, false
}

// describe says what a decoded JSON value is, for error messages: "null", "an
// array", "a string"…; "nothing" for a field that is absent.
func describe(value any, present bool) string {
	if !present {
		return "nothing"
	}

	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "a string"
	case float64:
		return "a number"
	case bool:
		return "a boolean"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	default:
		return fmt.Sprintf("a %T", value)
	}
}
