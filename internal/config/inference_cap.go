package config

import (
	"errors"
	"fmt"
)

// Default caps of inferences in flight (HU-IA.10). Local is what was measured as healthy
// against one Ollama (one generation at a time); API is a ceiling on spend and requests.
const (
	DefaultLocalMaxInFlight = 1
	DefaultAPIMaxInFlight   = 4
)

// ErrInvalidInferenceCap marks a negative llm.*.max_in_flight: the worker does not start.
var ErrInvalidInferenceCap = errors.New("invalid inference cap")

// InferenceCaps returns the caps of inferences in flight of the local model and of the
// paid provider. Zero or absent means the default.
func (c LLMConfig) InferenceCaps() (local, api int) {
	local, api = c.Local.MaxInFlight, c.API.MaxInFlight
	if local == 0 {
		local = DefaultLocalMaxInFlight
	}
	if api == 0 {
		api = DefaultAPIMaxInFlight
	}
	return local, api
}

// validateInferenceCaps rejects a negative cap with a typed error.
func (c LLMConfig) validateInferenceCaps() error {
	if c.Local.MaxInFlight < 0 {
		return fmt.Errorf("%w: llm.local.max_in_flight=%d (LLM_LOCAL_MAX_IN_FLIGHT) no puede ser negativo",
			ErrInvalidInferenceCap, c.Local.MaxInFlight)
	}
	if c.API.MaxInFlight < 0 {
		return fmt.Errorf("%w: llm.api.max_in_flight=%d (LLM_API_MAX_IN_FLIGHT) no puede ser negativo",
			ErrInvalidInferenceCap, c.API.MaxInFlight)
	}
	return nil
}
