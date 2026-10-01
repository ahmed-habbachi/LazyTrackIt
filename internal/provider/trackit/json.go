package trackit

import "encoding/json"

// looseString unmarshals a JSON string, number, or bool into a Go string.
// The OpenAPI spec declares several "color" fields as nullable strings, but
// at least one real TrackIt deployment returns them as numbers (likely a
// packed RGB int). Rather than trust the spec, we accept whatever scalar
// comes back and stringify it.
type looseString string

func (s *looseString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = looseString(str)
		return nil
	}
	// Numbers, bools, etc: the raw JSON token is already a fine string form.
	*s = looseString(data)
	return nil
}
