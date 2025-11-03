# Changelog

Todos los cambios notables en edugo-worker serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2025-11-03

### Tipo de Release: patch

- fix: corregir workflow sync-main-to-dev (#5)
- Dev (#4)

---

## [0.1.0] - 2025-11-01

### Added
- Sistema GitFlow profesional implementado
- Workflows de CI/CD automatizados:
  - CI Pipeline con tests y validaciones
  - Tests con cobertura y servicios de infraestructura
  - Manual Release workflow (TODO-EN-UNO) para control total de releases
  - Docker only workflow para builds manuales
  - Release automático con versionado semántico
- GitHub Copilot custom instructions en español (adaptado para workers)
- Migración a edugo-shared con arquitectura modular
- Submódulos: common, logger, database/postgres
- .gitignore completo para Go
- Documentación completa de workflows
- Patrones específicos de workers (processors, retry logic)

### Changed
- Actualizado a Go 1.25.3
- Versionado corregido a v0.x.x (proyecto en desarrollo)
- Eliminado auto-version.yml (reemplazado por manual-release.yml)
- Adaptaciones específicas para workers (sin HTTP handlers)

### Fixed
- Corrección de errores de linter (errcheck)
- Permisos de GitHub Container Registry configurados

[Unreleased]: https://github.com/EduGoGroup/edugo-worker/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EduGoGroup/edugo-worker/releases/tag/v0.1.0
