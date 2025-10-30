# ============================================
# Makefile - API Mobile (EduGo)
# ============================================

# Variables
APP_NAME=worker
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=bin
COVERAGE_DIR=coverage
MAIN_PATH=./cmd/main.go

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Colors
YELLOW=\033[1;33m
GREEN=\033[1;32m
BLUE=\033[1;34m
RED=\033[1;31m
RESET=\033[0m

# Environment variables con defaults
export APP_ENV ?= local
export POSTGRES_PASSWORD ?= edugo_pass
export MONGODB_URI ?= mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
export RABBITMQ_URL ?= amqp://edugo_user:edugo_pass@localhost:5672/

.DEFAULT_GOAL := help

# ============================================
# Main Targets
# ============================================

help: ## Mostrar esta ayuda
	@echo "$(BLUE)$(APP_NAME) - Comandos disponibles:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'

build: ## Compilar binario
	@echo "$(YELLOW)🔨 Compilando $(APP_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Binario: $(BUILD_DIR)/$(APP_NAME) ($(VERSION))$(RESET)"

run: ## Ejecutar en modo desarrollo
	@echo "$(YELLOW)🚀 Ejecutando $(APP_NAME) (ambiente: $(APP_ENV))...$(RESET)"
	@$(GOCMD) run $(MAIN_PATH)

dev: deps swagger run ## Desarrollo completo

# ============================================
# Testing
# ============================================

test: ## Ejecutar todos los tests
	@echo "$(YELLOW)🧪 Ejecutando tests...$(RESET)"
	@$(GOTEST) -v -race ./...
	@echo "$(GREEN)✓ Tests completados$(RESET)"

test-coverage: ## Tests con cobertura (HTML report)
	@echo "$(YELLOW)📊 Generando reporte de cobertura...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1
	@echo "$(GREEN)✓ Reporte: $(COVERAGE_DIR)/coverage.html$(RESET)"
	@echo "$(BLUE)💡 Abrir: open $(COVERAGE_DIR)/coverage.html$(RESET)"

test-unit: ## Solo tests unitarios
	@$(GOTEST) -v -short ./...

test-integration: ## Tests de integración
	@$(GOTEST) -v -tags=integration ./...

benchmark: ## Ejecutar benchmarks
	@echo "$(YELLOW)⚡ Ejecutando benchmarks...$(RESET)"
	@$(GOTEST) -bench=. -benchmem ./...

# ============================================
# Code Quality
# ============================================

fmt: ## Formatear código
	@echo "$(YELLOW)✨ Formateando código...$(RESET)"
	@$(GOFMT) -w .
	@echo "$(GREEN)✓ Código formateado$(RESET)"

vet: ## Análisis estático
	@echo "$(YELLOW)🔍 Ejecutando go vet...$(RESET)"
	@$(GOVET) ./...
	@echo "$(GREEN)✓ Análisis estático completado$(RESET)"

lint: ## Linter completo
	@echo "$(YELLOW)🔎 Ejecutando golangci-lint...$(RESET)"
	@golangci-lint run --timeout=5m || echo "$(YELLOW)⚠️  Instalar con: make tools$(RESET)"

audit: ## Auditoría de calidad completa
	@echo "$(BLUE)=== 🔐 AUDITORÍA ===$(RESET)"
	@echo "$(YELLOW)1. Verificando go.mod...$(RESET)"
	@$(GOMOD) verify
	@echo "$(YELLOW)2. Formato...$(RESET)"
	@test -z "$$($(GOFMT) -l .)" || (echo "$(RED)Sin formatear:$(RESET)" && $(GOFMT) -l .)
	@echo "$(YELLOW)3. Vet...$(RESET)"
	@$(GOVET) ./...
	@echo "$(YELLOW)4. Tests...$(RESET)"
	@$(GOTEST) -race -vet=off ./...
	@echo "$(GREEN)✓ Auditoría completada$(RESET)"

# ============================================
# Dependencies
# ============================================

deps: ## Descargar dependencias
	@echo "$(YELLOW)📦 Instalando dependencias...$(RESET)"
	@$(GOMOD) download
	@echo "$(GREEN)✓ Dependencias listas$(RESET)"

tidy: ## Limpiar go.mod
	@echo "$(YELLOW)🧹 Limpiando go.mod...$(RESET)"
	@$(GOMOD) tidy
	@echo "$(GREEN)✓ go.mod actualizado$(RESET)"

tools: ## Instalar herramientas
	@echo "$(YELLOW)🛠️  Instalando herramientas...$(RESET)"
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)✓ Herramientas instaladas$(RESET)"

# ============================================
# Swagger
# ============================================

swagger: ## Regenerar Swagger
	@echo "$(YELLOW)📚 Regenerando Swagger...$(RESET)"
	@swag init -g cmd/main.go -o docs --parseInternal
	@echo "$(GREEN)✓ Swagger: http://localhost:none/swagger/index.html$(RESET)"

# ============================================
# Docker
# ============================================

docker-build: ## Build imagen
	@echo "$(YELLOW)🐳 Building...$(RESET)"
	@docker build -t edugo/$(APP_NAME):$(VERSION) .
	@echo "$(GREEN)✓ Imagen: edugo/$(APP_NAME):$(VERSION)$(RESET)"

docker-run: ## Run con compose
	@docker-compose up -d
	@echo "$(GREEN)✓ Corriendo en http://localhost:none$(RESET)"

docker-stop: ## Stop compose
	@docker-compose down

docker-logs: ## Ver logs
	@docker-compose logs -f

# ============================================
# CI/CD
# ============================================

ci: audit test-coverage swagger ## CI pipeline
	@echo "$(GREEN)✅ CI completado$(RESET)"

pre-commit: fmt vet test ## Pre-commit hook

# ============================================
# Cleanup
# ============================================

clean: ## Limpiar todo
	@rm -rf $(BUILD_DIR) $(COVERAGE_DIR)
	@$(GOCMD) clean -cache -testcache
	@echo "$(GREEN)✓ Limpieza completa$(RESET)"

# ============================================
# Meta
# ============================================

all: clean deps fmt vet swagger test build ## Build completo
	@echo "$(GREEN)🎉 Build completo$(RESET)"

info: ## Info del proyecto
	@echo "$(BLUE)📋 $(APP_NAME)$(RESET)"
	@echo "  Versión: $(VERSION)"
	@echo "  Ambiente: $(APP_ENV)"
	@echo "  Go: $$($(GOCMD) version)"

.PHONY: help build run dev test test-coverage test-unit test-integration benchmark fmt vet lint audit deps tidy tools swagger docker-build docker-run docker-stop docker-logs ci pre-commit clean all info
export OPENAI_API_KEY ?= sk-test-key
