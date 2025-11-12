package adapter

import (
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/sirupsen/logrus"
)

// LoggerAdapter adapta *logrus.Logger (de shared/bootstrap) a logger.Logger (interfaz de shared/logger)
// Este adapter permite que el código de api-mobile siga usando la interfaz logger.Logger
// mientras internamente usamos *logrus.Logger retornado por shared/bootstrap
type LoggerAdapter struct {
	logrus *logrus.Logger
}

// NewLoggerAdapter crea un nuevo adapter de logger
func NewLoggerAdapter(logrusLogger *logrus.Logger) logger.Logger {
	return &LoggerAdapter{
		logrus: logrusLogger,
	}
}

// Debug registra un mensaje de nivel debug
// Convierte los fields de interface{} a logrus.Fields
func (a *LoggerAdapter) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		a.logrus.WithFields(convertToLogrusFields(fields...)).Debug(msg)
	} else {
		a.logrus.Debug(msg)
	}
}

// Info registra un mensaje de nivel info
func (a *LoggerAdapter) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		a.logrus.WithFields(convertToLogrusFields(fields...)).Info(msg)
	} else {
		a.logrus.Info(msg)
	}
}

// Warn registra un mensaje de nivel warning
func (a *LoggerAdapter) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		a.logrus.WithFields(convertToLogrusFields(fields...)).Warn(msg)
	} else {
		a.logrus.Warn(msg)
	}
}

// Error registra un mensaje de nivel error
func (a *LoggerAdapter) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		a.logrus.WithFields(convertToLogrusFields(fields...)).Error(msg)
	} else {
		a.logrus.Error(msg)
	}
}

// Fatal registra un mensaje de nivel fatal y termina la aplicación
func (a *LoggerAdapter) Fatal(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		a.logrus.WithFields(convertToLogrusFields(fields...)).Fatal(msg)
	} else {
		a.logrus.Fatal(msg)
	}
}

// With agrega campos contextuales al logger y retorna un nuevo logger
// Útil para agregar información de contexto que se incluirá en todos los logs
func (a *LoggerAdapter) With(fields ...interface{}) logger.Logger {
	// Crear un nuevo Entry con los campos adicionales
	entry := a.logrus.WithFields(convertToLogrusFields(fields...))
	
	// Retornar un nuevo adapter que envuelva el entry
	return &loggerEntryAdapter{
		entry: entry,
	}
}

// Sync sincroniza el buffer del logger
// Logrus no requiere sync explícito, retornamos nil
func (a *LoggerAdapter) Sync() error {
	return nil
}

// loggerEntryAdapter adapta *logrus.Entry a logger.Logger
// Se usa cuando se llama With() para crear un logger con contexto
type loggerEntryAdapter struct {
	entry *logrus.Entry
}

// Debug registra un mensaje de nivel debug usando el entry
func (e *loggerEntryAdapter) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		e.entry.WithFields(convertToLogrusFields(fields...)).Debug(msg)
	} else {
		e.entry.Debug(msg)
	}
}

// Info registra un mensaje de nivel info usando el entry
func (e *loggerEntryAdapter) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		e.entry.WithFields(convertToLogrusFields(fields...)).Info(msg)
	} else {
		e.entry.Info(msg)
	}
}

// Warn registra un mensaje de nivel warning usando el entry
func (e *loggerEntryAdapter) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		e.entry.WithFields(convertToLogrusFields(fields...)).Warn(msg)
	} else {
		e.entry.Warn(msg)
	}
}

// Error registra un mensaje de nivel error usando el entry
func (e *loggerEntryAdapter) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		e.entry.WithFields(convertToLogrusFields(fields...)).Error(msg)
	} else {
		e.entry.Error(msg)
	}
}

// Fatal registra un mensaje de nivel fatal usando el entry
func (e *loggerEntryAdapter) Fatal(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		e.entry.WithFields(convertToLogrusFields(fields...)).Fatal(msg)
	} else {
		e.entry.Fatal(msg)
	}
}

// With agrega más campos contextuales al logger
func (e *loggerEntryAdapter) With(fields ...interface{}) logger.Logger {
	entry := e.entry.WithFields(convertToLogrusFields(fields...))
	return &loggerEntryAdapter{
		entry: entry,
	}
}

// Sync sincroniza el buffer del logger
func (e *loggerEntryAdapter) Sync() error {
	return nil
}

// convertToLogrusFields convierte los campos variadic interface{} a logrus.Fields
// Espera pares key-value: convertToLogrusFields("key1", "value1", "key2", value2)
func convertToLogrusFields(fields ...interface{}) logrus.Fields {
	logrusFields := make(logrus.Fields)
	
	// Procesar pares key-value
	for i := 0; i < len(fields)-1; i += 2 {
		// La key debe ser string
		key, ok := fields[i].(string)
		if !ok {
			// Si no es string, convertir a string
			key = "unknown"
		}
		
		// El value puede ser cualquier tipo
		value := fields[i+1]
		logrusFields[key] = value
	}
	
	// Si hay un número impar de campos, el último se ignora
	// (esto sigue el comportamiento estándar de loggers estructurados)
	
	return logrusFields
}

// Verificar en compile-time que LoggerAdapter implementa logger.Logger
var _ logger.Logger = (*LoggerAdapter)(nil)
var _ logger.Logger = (*loggerEntryAdapter)(nil)
