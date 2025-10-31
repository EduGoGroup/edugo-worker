# Changelog

Todos los cambios notables en edugo-worker serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-10-31

### Added
- Sistema GitFlow profesional implementado
- Workflows de CI/CD automatizados:
  - CI Pipeline con tests y validaciones
  - Tests con cobertura y servicios de infraestructura
  - Build y push automático de Docker images
  - Release automático con versionado semántico
  - Sincronización automática main ↔ dev
- Auto-versionado basado en Conventional Commits
- Migración a edugo-shared v2.0.5 con arquitectura modular
- Submódulos: common, logger, database/postgres
- .gitignore completo para Go
- Documentación completa de workflows

### Changed
- Actualizado a Go 1.25.3
- Optimización de dependencias (reducción ~80%)

### Fixed
- Corrección de errores de linter (errcheck)
- Permisos de GitHub Container Registry configurados

[Unreleased]: https://github.com/EduGoGroup/edugo-worker/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/EduGoGroup/edugo-worker/releases/tag/v1.0.0
