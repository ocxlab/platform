package httpapi

import (
	"errors"
	"testing"

	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
)

func TestDisabledDomainDoesNotInstallOfferingImplementation(t *testing.T) {
	var server *Server
	_, err := NewHandler(nil, WithDomains("none"), func(s *Server) { server = s })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.models.Validate(food.Model, nil); !errors.Is(err, offering.ErrUnsupported) {
		t.Fatalf("disabled model validator installed: %v", err)
	}
	if _, err = server.models.Project(food.Model, nil); !errors.Is(err, offering.ErrUnsupported) {
		t.Fatalf("disabled model projection installed: %v", err)
	}
}
