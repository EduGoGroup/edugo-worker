//go:build integration
// +build integration

package integration

import (
	"os"
	"strconv"
	"testing"
)

// ShouldRunIntegrationTests verifica si los tests de integración deben ejecutarse
// Retorna true si:
// 1. RUN_INTEGRATION_TESTS=true (variable de entorno)
// 2. INTEGRATION_TESTS=1 (variable de entorno alternativa)
// 3. CI=true (en ambiente CI siempre corre, a menos que esté deshabilitado explícitamente)
func ShouldRunIntegrationTests() bool {
	// Check RUN_INTEGRATION_TESTS
	if val := os.Getenv("RUN_INTEGRATION_TESTS"); val != "" {
		run, _ := strconv.ParseBool(val)
		return run
	}

	// Check INTEGRATION_TESTS (formato alternativo)
	if val := os.Getenv("INTEGRATION_TESTS"); val == "1" || val == "true" {
		return true
	}

	// En CI, correr por defecto (a menos que esté explícitamente deshabilitado)
	if os.Getenv("CI") == "true" {
		// Pero permitir deshabilitar con SKIP_INTEGRATION_TESTS
		if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
			return false
		}
		return true
	}

	// Localmente, NO correr por defecto (deben habilitarse explícitamente)
	return false
}

// SkipIfIntegrationTestsDisabled salta el test si los tests de integración están deshabilitados
// Usar al inicio de cada test de integración:
//
//	func TestSomething(t *testing.T) {
//	    integration.SkipIfIntegrationTestsDisabled(t)
//	    // ... resto del test
//	}
func SkipIfIntegrationTestsDisabled(t *testing.T) {
	t.Helper()
	if !ShouldRunIntegrationTests() {
		t.Skip("⏭️  Integration tests disabled. Set RUN_INTEGRATION_TESTS=true to enable")
	}
}
