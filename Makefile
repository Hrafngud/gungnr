COMPOSE = docker compose

.PHONY: up up-d down down-v logs ps restart build

up: ## Build and start the stack (foreground)
	$(COMPOSE) up --build

up-d: ## Build and start the stack (detached)
	$(COMPOSE) up --build -d

build: ## Build images
	$(COMPOSE) build

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
