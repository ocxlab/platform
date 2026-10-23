// Package runtimeconfig selects deployment composition without changing domain code.
package runtimeconfig

import "errors"

type Process struct {
	Role    string
	Service string
	Domains string
}

// Resolve retains the public/internal/worker compositions used by local network
// experiments and adds explicit, independently deployable service selections.
// A contradictory environment must fail rather than silently enable extra APIs.
func Resolve(service, role, domains string) (Process, error) {
	p := Process{Role: role, Service: "combined", Domains: domains}
	if role != "public" && role != "internal" && role != "worker" {
		return Process{}, errors.New("unknown process role")
	}
	if service == "" {
		return p, nil
	}
	if role != "public" {
		return Process{}, errors.New("select either a service or a non-default role")
	}
	p.Service = service
	switch service {
	case "combined", "core":
		// Domain selection controls the installed catalog models. Core does not
		// install Food purchase routes, even when its catalog accepts Food menus.
	case "food":
		if domains != "" && domains != "food" {
			return Process{}, errors.New("Food service requires only the Food domain")
		}
		p.Domains = "food"
	case "shopping", "lodging":
		if domains != "" && domains != service {
			return Process{}, errors.New("service and enabled domain disagree")
		}
		p.Role, p.Domains = "internal", service
	default:
		return Process{}, errors.New("unknown service")
	}
	return p, nil
}
