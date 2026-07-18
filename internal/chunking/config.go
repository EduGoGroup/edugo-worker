// Package chunking implementa el porcionado determinista de texto (D-043.6):
// parte un documento en trozos ("chunks") por encabezados y líneas en blanco,
// apuntando a un tamaño objetivo por trozo, sin solape entre ellos. La
// continuidad semántica entre trozos la aporta el resumen encadenado del
// pipeline, no el solape.
//
// La función principal Split es PURA y DETERMINISTA: para el mismo texto y la
// misma Config produce exactamente el mismo resultado. No usa aleatoriedad,
// reloj, IO ni logging.
package chunking

import "fmt"

// Config controla el porcionado. Todos los valores están en palabras y son
// parámetros (no constantes) para poder afinarlos desde configuración.
type Config struct {
	// TargetWords es el tamaño deseado por trozo. Al alcanzarlo, el siguiente
	// bloque abre un trozo nuevo.
	TargetWords int
	// MaxWords es el tope duro por trozo durante el empaquetado. Solo la fusión
	// de restos chicos puede superarlo levemente.
	MaxWords int
	// MinWords es el mínimo para considerar "completo" un trozo: un encabezado
	// solo fuerza corte si el trozo actual ya alcanzó este mínimo.
	MinWords int
	// MergeThresholdWords es el piso efectivo de un trozo: los restos por debajo
	// de este valor se fusionan con el vecino.
	MergeThresholdWords int
}

// DefaultConfig devuelve los parámetros por defecto del contrato de diseño:
// objetivo ~650, máximo 800, mínimo 500, umbral de fusión 150 palabras.
func DefaultConfig() Config {
	return Config{
		TargetWords:         650,
		MaxWords:            800,
		MinWords:            500,
		MergeThresholdWords: 150,
	}
}

// Validate comprueba que la Config sea coherente. Split no la exige (normaliza
// valores inválidos a los defaults), pero se expone para quien quiera validar
// explícitamente antes de invocar.
func (c Config) Validate() error {
	if c.TargetWords <= 0 || c.MaxWords <= 0 || c.MinWords <= 0 || c.MergeThresholdWords <= 0 {
		return fmt.Errorf("chunking: todos los parámetros deben ser mayores que 0")
	}
	if !(c.MinWords <= c.TargetWords && c.TargetWords <= c.MaxWords) {
		return fmt.Errorf("chunking: se requiere MinWords <= TargetWords <= MaxWords")
	}
	if c.MergeThresholdWords > c.MinWords {
		return fmt.Errorf("chunking: MergeThresholdWords no puede superar MinWords")
	}
	return nil
}

// normalized devuelve una copia de la Config con los valores inválidos
// reemplazados por defaults y el orden Min <= Target <= Max asegurado. Se aplica
// al inicio de Split para que la función nunca panique ni produzca trozos
// absurdos ante una Config mal armada.
func (c Config) normalized() Config {
	d := DefaultConfig()
	if c.TargetWords <= 0 {
		c.TargetWords = d.TargetWords
	}
	if c.MaxWords <= 0 {
		c.MaxWords = d.MaxWords
	}
	if c.MinWords <= 0 {
		c.MinWords = d.MinWords
	}
	if c.MergeThresholdWords <= 0 {
		c.MergeThresholdWords = d.MergeThresholdWords
	}
	// Asegurar Min <= Target <= Max reajustando los extremos que se salgan.
	// El orden importa: primero se acota Target contra Max y recién después Min
	// contra Target, para que un Max chico arrastre a ambos.
	if c.TargetWords > c.MaxWords {
		c.TargetWords = c.MaxWords
	}
	if c.MinWords > c.TargetWords {
		c.MinWords = c.TargetWords
	}
	// El umbral de fusión nunca debe superar el mínimo del trozo.
	if c.MergeThresholdWords > c.MinWords {
		c.MergeThresholdWords = c.MinWords
	}
	return c
}
