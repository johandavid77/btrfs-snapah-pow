# ═══════════════════════════════════════════════════════════
#  BTRFS SNAPAH POW - Makefile
# ═══════════════════════════════════════════════════════════

.PHONY: all build build-server build-agent build-cli clean test fmt lint proto help

APP_NAME   := snapah-pow
VERSION    := 0.1.0
BUILD_TIME := $(shell date +%Y-%m-%d_%H:%M:%S)
GO         := go

SERVER_BIN := bin/snapah-server
AGENT_BIN  := bin/snapah-agent
CLI_BIN    := bin/snapah

# ─── Colores ───────────────────────────────────────────────
GREEN  := $(shell tput setaf 2)
CYAN   := $(shell tput setaf 6)
YELLOW := $(shell tput setaf 3)
RESET  := $(shell tput sgr0)

# ─── Targets ───────────────────────────────────────────────

all: build

build: build-server build-agent build-cli

build-server:
	@echo "$(CYAN)🔥 Compilando Snapah Pow Server...$(RESET)"
	@mkdir -p bin
	$(GO) build -ldflags "-X main.appVersion=$(VERSION)" -o $(SERVER_BIN) ./cmd/server
	@echo "$(GREEN)✅ Server listo$(RESET)"

build-agent:
	@echo "$(CYAN)🔥 Compilando Snapah Pow Agent...$(RESET)"
	@mkdir -p bin
	$(GO) build -ldflags "-X main.appVersion=$(VERSION)" -o $(AGENT_BIN) ./cmd/agent
	@echo "$(GREEN)✅ Agent listo$(RESET)"

build-cli:
	@echo "$(CYAN)🔥 Compilando Snapah Pow CLI...$(RESET)"
	@mkdir -p bin
	$(GO) build -ldflags "-X main.appVersion=$(VERSION)" -o $(CLI_BIN) ./cmd/cli
	@echo "$(GREEN)✅ CLI listo$(RESET)"

clean:
	rm -rf bin/

help:
	@echo ""
	@echo "$(CYAN)╔══════════════════════════════════════════════════════╗$(RESET)"
	@echo "$(CYAN)║        🔥 BTRFS SNAPAH POW - Comandos               ║$(RESET)"
	@echo "$(CYAN)╚══════════════════════════════════════════════════════╝$(RESET)"
	@echo ""
	@echo "  $(YELLOW)build$(RESET)        Compila server + agent + cli"
	@echo "  $(YELLOW)build-server$(RESET) Compila solo el servidor"
	@echo "  $(YELLOW)build-agent$(RESET)  Compila solo el agente"
	@echo "  $(YELLOW)build-cli$(RESET)    Compila solo el CLI"
	@echo "  $(YELLOW)clean$(RESET)        Elimina binarios"
	@echo "  $(YELLOW)help$(RESET)         Muestra esta ayuda"
	@echo ""
