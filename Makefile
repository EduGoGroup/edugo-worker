# ============================================
# Makefile - edugo-worker (EduGo)
# ============================================

# Variables
APP_NAME=worker
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=bin
COVERAGE_DIR=coverage
MAIN_PATH=./cmd

# Versions (keep in sync with .github/workflows/ci.yml)
GO_VERSION  := 1.27.4
LINT_VERSION := v2.14.0
GOVULNCHECK_VERSION := v1.9.0

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

# Environment variables con defaults.
# Plan 037 (D-037.11): worker esqueleto sin Postgres ni Mongo; su única
# dependencia es RabbitMQ (edugo-rabbitmq 5672). Con estos defaults,
# `APP_ENV=local make run` arranca sin exportar nada a mano.
# Plan 054: todos los targets compilan como la CI, sin heredar un go.work del árbol padre.
export GOWORK := off

export APP_ENV ?= local
export RABBITMQ_URL ?= amqp://guest:guest@localhost:5672/

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

dev: deps run ## Desarrollo completo

llm-harness: ## Smoke del provider LLM: mode=generate (039 D-039.8, contrato 038) o mode=review (040 T2c, corrección). Flags via ARGS="..."
	@echo "$(YELLOW)🧪 llm-harness (provider LLM: generate=contrato 038 · review=corrección de respuestas)...$(RESET)"
	@echo "$(YELLOW)   ej: make llm-harness ARGS=\"-mode review -provider ollama -model qwen3:1.7b\"$(RESET)"
	@$(GOCMD) run ./cmd/llm-harness $(ARGS)

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

# Gate local obligatorio antes de cada PR (plan 054, S.6): la integración no corre en CI.
# Fuerza GOWORK=off y RUN_INTEGRATION_TESTS=true para que no dependa del entorno de quien lo lanza.
test-integration: ## Tests de integración (gate local previo al PR; requiere Docker)
	@docker info > /dev/null 2>&1 || { echo "$(RED)✗ Docker no está disponible: los tests de integración usan testcontainers$(RESET)"; exit 1; }
	@echo "$(YELLOW)🧪 Ejecutando tests de integración (GOWORK=off, RUN_INTEGRATION_TESTS=true)...$(RESET)"
	@RUN_INTEGRATION_TESTS=true $(GOTEST) -v -race -tags=integration ./...
	@echo "$(GREEN)✓ Tests de integración completados$(RESET)"

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

lint: ## Linter completo (falla si golangci-lint no está instalado o si hay hallazgos)
	@echo "$(YELLOW)🔎 Ejecutando golangci-lint...$(RESET)"
	@command -v golangci-lint > /dev/null 2>&1 || { echo "$(RED)✗ golangci-lint no está instalado. Instalar con: make tools$(RESET)"; exit 1; }
	@golangci-lint run --allow-parallel-runners --timeout=5m ./...
	@echo "$(GREEN)✓ Lint sin hallazgos$(RESET)"

vuln: ## Vulnerabilidades alcanzables con govulncheck ($(GOVULNCHECK_VERSION))
	@echo "$(YELLOW)🛡️  Ejecutando govulncheck...$(RESET)"
	@go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	@echo "$(GREEN)✓ Sin vulnerabilidades alcanzables$(RESET)"

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
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
	@echo "$(GREEN)✓ Herramientas instaladas$(RESET)"

# ============================================
# Docker
# ============================================

docker-build: ## Build imagen
	@echo "$(YELLOW)🐳 Building...$(RESET)"
	@docker build -t edugo/$(APP_NAME):$(VERSION) .
	@echo "$(GREEN)✓ Imagen: edugo/$(APP_NAME):$(VERSION)$(RESET)"

docker-run: ## Run con compose
	@docker-compose up -d
	@echo "$(GREEN)✓ Worker corriendo (sin HTTP de negocio; métricas en :9090)$(RESET)"

docker-stop: ## Stop compose
	@docker-compose down

docker-logs: ## Ver logs
	@docker-compose logs -f

# ============================================
# CI/CD
# ============================================

ci: audit test-coverage ## CI pipeline
	@echo "$(GREEN)✅ CI completado$(RESET)"

ci-local: fmt vet lint test-unit vuln ## Pre-push: mismos checks que el CI de GitHub (con lint y govulncheck)
	@echo "$(GREEN)CI local OK$(RESET)"

ci-docker: ## Simula el CI en Docker (Go $(GO_VERSION) + golangci-lint $(LINT_VERSION) + govulncheck $(GOVULNCHECK_VERSION)) — requiere Docker
	@which docker > /dev/null 2>&1 || (echo "$(RED)Docker no instalado$(RESET)" && exit 1)
	@echo "$(YELLOW)Ejecutando CI en Docker (Go $(GO_VERSION))...$(RESET)"
	@docker run --rm \
		-e GOPRIVATE=github.com/EduGoGroup/* \
		-e GOWORK=off \
		-v "$(HOME)/.netrc:/root/.netrc:ro" \
		-v "$$(go env GOPATH)/pkg/mod:/go/pkg/mod" \
		-v "$(PWD):/workspace" \
		-w /workspace \
		golang:$(GO_VERSION)-bookworm \
		bash -c "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin $(LINT_VERSION) && go vet ./... && golangci-lint run --timeout=5m && go test -short -race ./... && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./..."
	@echo "$(GREEN)CI Docker OK$(RESET)"

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

all: clean deps fmt vet test build ## Build completo
	@echo "$(GREEN)🎉 Build completo$(RESET)"

info: ## Info del proyecto
	@echo "$(BLUE)📋 $(APP_NAME)$(RESET)"
	@echo "  Versión: $(VERSION)"
	@echo "  Ambiente: $(APP_ENV)"
	@echo "  Go: $$($(GOCMD) version)"

.PHONY: help build run dev test test-coverage test-unit test-integration benchmark fmt vet lint vuln audit deps tidy tools docker-build docker-run docker-stop docker-logs ci ci-local ci-docker pre-commit clean all info
export OPENAI_API_KEY ?= sk-test-key
