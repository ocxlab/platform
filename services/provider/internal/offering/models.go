// Package offering owns replaceable catalog-model validation, not purchases.
package offering

import (
	"encoding/json"
	"errors"
)

type Model struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
type Validator func(json.RawMessage) (json.RawMessage, error)
type Registry struct{ validators map[Model]Validator }

var ErrUnsupported = errors.New("unsupported offering model")
var ErrInvalid = errors.New("invalid offering definition")

// NewRegistry copies installed implementations before the service starts.
func NewRegistry(installed map[Model]Validator) *Registry {
	copy := make(map[Model]Validator, len(installed))
	for key, value := range installed {
		copy[key] = value
	}
	return &Registry{validators: copy}
}

func (r *Registry) Validate(model Model, document json.RawMessage) (json.RawMessage, error) {
	validator, ok := r.validators[model]
	if !ok {
		return nil, ErrUnsupported
	}
	return validator(document)
}
