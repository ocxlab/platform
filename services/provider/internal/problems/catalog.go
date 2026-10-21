// Package problems owns safe public error metadata, not internal exception text.
package problems

import (
	_ "embed"
	"encoding/json"
)

//go:embed catalog.generated.json
var raw []byte

type Definition struct {
	Module     string `json:"module"`
	Status     int    `json:"status"`
	Title      string `json:"title"`
	MessageKey string `json:"message_key"`
}

var catalog = func() map[string]Definition {
	var value map[string]Definition
	if err := json.Unmarshal(raw, &value); err != nil {
		panic("invalid generated error catalog")
	}
	return value
}()

func Lookup(code string) (Definition, bool) { value, ok := catalog[code]; return value, ok }
