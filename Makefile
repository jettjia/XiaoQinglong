# ================================================
# XiaoQinglong Project Makefile
# ================================================

# Go projects
AGENT_FRAME := backend/agent-frame
RUNNER := backend/runner
AGENT_UI := frontend/agent-ui

# Docker image names
AGENT_FRAME_IMAGE := xiaoqinglong/agent-frame:latest
RUNNER_IMAGE := xiaoqinglong/runner:latest
AGENT_UI_IMAGE := xiaoqinglong/agent-ui:latest

# Mock services
MOCK_SERVICES := a2a http kb-service mcp

# Docker deploy directory
DEPLOY_DIR := deploy/docker

# ================================================
# Build Targets
# ================================================

.PHONY: build build-frame build-runner build-ui build-all
build: build-frame build-runner build-ui

build-frame:
	@echo "Building agent-frame..."
	@mkdir -p $(AGENT_FRAME)/deploy/bin
	@GOWORK=off CGO_ENABLED=0 go build -C $(AGENT_FRAME) -mod=mod -ldflags="-s -w" -o deploy/bin/go-main .
	@if command -v upx > /dev/null 2>&1; then upx -9 $(AGENT_FRAME)/deploy/bin/go-main; else echo "UPX not found, skipping compression"; fi

build-runner:
	@echo "Building runner..."
	@cd $(RUNNER) && GOWORK=off CGO_ENABLED=0 go build -mod=mod -ldflags="-s -w" -o bin/runner .

build-ui:
	@echo "Building agent-ui..."
	@cd $(AGENT_UI) && npm run build

build-all: build-frame build-runner build-ui
	@echo "All builds completed!"

# ================================================
# Docker Build Targets
# ================================================

.PHONY: docker-build docker-build-all docker-push
docker-build: docker-build-frame docker-build-runner docker-build-ui

docker-build-frame:
	@echo "Building agent-frame docker image..."
	@docker build -t $(AGENT_FRAME_IMAGE) -f $(AGENT_FRAME)/deploy/Dockerfile-ez $(AGENT_FRAME)

docker-build-runner:
	@echo "Building runner docker image..."
	@docker build -t $(RUNNER_IMAGE) -f $(RUNNER)/Dockerfile $(RUNNER)

docker-build-ui:
	@echo "Building agent-ui docker image..."
	@docker build -t $(AGENT_UI_IMAGE) $(AGENT_UI)

docker-build-all: docker-build-frame docker-build-runner docker-build-ui
	@echo "All docker images built!"

docker-push:
	@echo "Pushing docker images..."
	@docker push $(AGENT_FRAME_IMAGE)
	@docker push $(RUNNER_IMAGE)
	@docker push $(AGENT_UI_IMAGE)

# ================================================
# Docker Compose for Mock Services
# ================================================

.PHONY: mock-start mock-stop mock-restart mock-status

mock-start:
	@echo "Starting mock services..."
	@for svc in $(MOCK_SERVICES); do \
		(cd mock/$$svc && echo "Starting $$svc..." && (go run main.go &)); \
	done
	@echo "Mock services started!"

mock-stop:
	@echo "Stopping mock services..."
	@lsof -ti:28080,28081,28082,28083 | xargs -r kill -9 2>/dev/null || true
	@echo "Mock services stopped!"

mock-restart: mock-stop mock-start

mock-status:
	@echo "Mock services status..."
	@if pgrep -f "mock" > /dev/null; then \
		echo "Mock services: RUNNING"; \
	else \
		echo "Mock services: STOPPED"; \
	fi

# ================================================
# Docker Deploy (Production) Targets
# ================================================

.PHONY: deploy-start deploy-stop deploy-restart deploy-logs deploy-status

deploy-start:
	@echo "Starting all services with docker-compose..."
	@cd $(DEPLOY_DIR) && ./start.sh

deploy-stop:
	@echo "Stopping all services..."
	@cd $(DEPLOY_DIR) && ./stop.sh

deploy-restart: deploy-stop deploy-start

deploy-logs:
	@echo "Showing logs for all services..."
	@cd $(DEPLOY_DIR) && docker compose logs -f

deploy-status:
	@echo "Status of all services..."
	@cd $(DEPLOY_DIR) && docker compose ps

deploy-rebuild: docker-build-all
	@echo "Rebuilding and restarting services..."
	@cd $(DEPLOY_DIR) && docker compose up -d --force-recreate

# ================================================
# Development Targets
# ================================================

.PHONY: dev quick-start dev-frame dev-runner dev-ui

quick-start: dev
dev: dev-frame dev-runner dev-ui

dev-frame:
	@echo "Starting agent-frame in dev mode..."
	@cd $(AGENT_FRAME) && go run main.go

dev-runner:
	@echo "Starting runner in dev mode..."
	@cd $(RUNNER) && go run main.go

dev-ui:
	@echo "Starting agent-ui in dev mode..."
	@cd $(AGENT_UI) && npm run dev

# ================================================
# Cleanup
# ================================================

.PHONY: clean clean-build clean-docker

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(AGENT_FRAME)/deploy/bin
	@rm -rf $(RUNNER)/bin
	@rm -rf $(AGENT_UI)/dist

clean-docker:
	@echo "Cleaning docker images..."
	@docker rmi $(AGENT_FRAME_IMAGE) $(RUNNER_IMAGE) $(AGENT_UI_IMAGE) 2>/dev/null || true

# ================================================
# Help
# ================================================

.PHONY: help
help:
	@echo "XiaoQinglong Project Makefile"
	@echo ""
	@echo "Build Targets:"
	@echo "  build          - Build all components (frame, runner, ui)"
	@echo "  build-frame    - Build agent-frame"
	@echo "  build-runner   - Build runner"
	@echo "  build-ui       - Build agent-ui"
	@echo ""
	@echo "Docker Targets:"
	@echo "  docker-build       - Build all docker images"
	@echo "  docker-build-frame  - Build agent-frame docker image"
	@echo "  docker-build-runner - Build runner docker image"
	@echo "  docker-build-ui     - Build agent-ui docker image"
	@echo "  docker-push         - Push all docker images"
	@echo ""
	@echo "Deploy Targets (with PostgreSQL):"
	@echo "  deploy-start       - Start all services via docker-compose"
	@echo "  deploy-stop        - Stop all services"
	@echo "  deploy-restart     - Restart all services"
	@echo "  deploy-logs        - Show logs for all services"
	@echo "  deploy-status      - Show status of all services"
	@echo "  deploy-rebuild     - Rebuild images and recreate containers"
	@echo ""
	@echo "Mock Service Targets:"
	@echo "  mock-start     - Start all mock services (go run main.go)"
	@echo "  mock-stop      - Stop all mock services"
	@echo "  mock-restart   - Restart all mock services"
	@echo "  mock-status    - Show mock services status"
	@echo ""
	@echo "Development Targets:"
	@echo "  dev             - Start all components in dev mode"
	@echo "  dev-frame       - Start agent-frame in dev mode"
	@echo "  dev-runner      - Start runner in dev mode"
	@echo "  dev-ui          - Start agent-ui in dev mode"
	@echo ""
	@echo "Cleanup Targets:"
	@echo "  clean           - Remove build artifacts"
	@echo "  clean-docker    - Remove docker images"
