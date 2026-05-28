//go:build integration
// +build integration

package repository_test

import (
	"testing"

	"github.com/EduGoGroup/edugo-worker/test/integration"
)

// SkipIfIntegrationTestsDisabled salta el test si los tests de integración están deshabilitados
// Esta función usa la implementación compartida de test/integration
//
// Usar al inicio de cada test de integración:
//
//	func TestSomething(t *testing.T) {
//	    SkipIfIntegrationTestsDisabled(t)
//	    // ... resto del test
//	}
func SkipIfIntegrationTestsDisabled(t *testing.T) {
	t.Helper()
	integration.SkipIfIntegrationTestsDisabled(t)
}
