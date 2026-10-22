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
type Registry struct {
	validators  map[Model]Validator
	projections map[Model]Validator
}

var ErrUnsupported = errors.New("unsupported offering model")
var ErrInvalid = errors.New("invalid offering definition")

func (r *Registry) Supports(model Model) bool {
	return r != nil && r.validators[model] != nil && r.projections[model] != nil
}

// NewRegistry copies installed implementations before the service starts.
func NewRegistry(installed map[Model]Validator, projections ...map[Model]Validator) *Registry {
	copy := make(map[Model]Validator, len(installed))
	for key, value := range installed {
		copy[key] = value
	}
	public := make(map[Model]Validator)
	for _, group := range projections {
		for key, value := range group {
			public[key] = value
		}
	}
	return &Registry{validators: copy, projections: public}
}

func (r *Registry) Project(model Model, document json.RawMessage) (json.RawMessage, error) {
	project, ok := r.projections[model]
	if !ok {
		return nil, ErrUnsupported
	}
	return project(document)
}

func (r *Registry) Validate(model Model, document json.RawMessage) (json.RawMessage, error) {
	validator, ok := r.validators[model]
	if !ok {
		return nil, ErrUnsupported
	}
	return validator(document)
}
