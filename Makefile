COMPOSE = docker compose
CLI_BUILD = ./scripts/build_gungnr.sh

.PHONY: up up-d down down-v logs ps restart build build-cli

up: ## Build and start the stack (foreground)
	$(COMPOSE) up --build

up-d: ## Build and start the stack (detached)
	$(COMPOSE) up --build -d

build: ## Build images
	$(COMPOSE) build

build-cli: ## Build the gungnr CLI with ldflags-backed version metadata
	$(CLI_BUILD)

down: ## Stop services
	$(COMPOSE) down

down-v: ## Stop services and remove volumes
	$(COMPOSE) down -v

logs: ## Tail service logs
	$(COMPOSE) logs -f --tail=200

ps: ## Show service status
	$(COMPOSE) ps

restart: ## Restart services
	$(COMPOSE) restart
