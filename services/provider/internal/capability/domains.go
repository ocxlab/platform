// Package capability separates installed domain support from runtime admission.
// A domain name does not imply a UCP capability or protocol conformance.
package capability

import (
	"errors"
	"strings"
)

type Domain string

const (
	Food     Domain = "food"
	Shopping Domain = "shopping"
	Lodging  Domain = "lodging"
)

func (d Domain) Valid() bool {
	return d == Food || d == Shopping || d == Lodging
}

// Domains is immutable after startup. Parse accepts only installed implementations;
// selecting a name cannot install handlers or invent advertised compatibility.
type Domains struct{ enabled map[Domain]bool }

var ErrConfiguration = errors.New("invalid commerce domain configuration")

// Parse uses defaults only when configuration is absent. Explicit "none" enables
// control-plane-only processes. Empty tokens and duplicate selections are errors.
func Parse(raw string, installed, defaults []Domain) (Domains, error) {
	available := make(map[Domain]bool, len(installed))
	for _, d := range installed {
		if !d.Valid() || available[d] {
			return Domains{}, ErrConfiguration
		}
		available[d] = true
	}
	selected := defaults
	if raw == "none" {
		selected = nil
	} else if raw != "" {
		selected = nil
		for _, token := range strings.Split(raw, ",") {
			selected = append(selected, Domain(strings.TrimSpace(token)))
		}
	}
	enabled := make(map[Domain]bool, len(selected))
	for _, d := range selected {
		if !available[d] || enabled[d] {
			return Domains{}, ErrConfiguration
		}
		enabled[d] = true
	}
	return Domains{enabled: enabled}, nil
}

func (d Domains) Allows(domain Domain) bool { return d.enabled[domain] }

func (d Domains) Enabled() []Domain {
	result := []Domain{}
	for _, domain := range []Domain{Food, Shopping, Lodging} {
		if d.Allows(domain) {
			result = append(result, domain)
		}
	}
	return result
}
