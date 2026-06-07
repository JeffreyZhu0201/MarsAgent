.PHONY: help infra-up infra-down infra-logs dev gateway agents web proto test fmt

help:
	@echo "Targets:"
	@echo "  infra-up      启动 postgres/redis/qdrant/minio"
	@echo "  infra-down    停掉基础设施"
	@echo "  infra-logs    跟随基础设施日志"
	@echo "  dev           提示如何并行启动三服务"
	@echo "  gateway       启动 Go gateway (前台)"
	@echo "  agents        启动 Python agents worker (前台)"
	@echo "  web           启动 Vite 前端 (前台)"
	@echo "  proto         重新生成 proto 代码 (Go + Python)"
	@echo "  test          运行全部测试"
	@echo "  fmt           格式化全部代码"

infra-up:
	docker compose -f infra/docker-compose.dev.yml --env-file infra/.env.example up -d

infra-down:
	docker compose -f infra/docker-compose.dev.yml down

infra-logs:
	docker compose -f infra/docker-compose.dev.yml logs -f

dev:
	@echo "在三个终端分别运行:"
	@echo "  make gateway"
	@echo "  make agents"
	@echo "  make web"

gateway:
	cd apps/gateway && go run ./cmd/server

agents:
	cd apps/agents && uvicorn marsagent.main:app --reload --port 8001

web:
	cd apps/web && npm run dev

proto:
	cd proto && bash ../scripts/gen-proto.sh

test:
	cd apps/gateway && go test ./... && \
	cd ../agents && pytest -q && \
	cd ../web && npm test --silent

fmt:
	cd apps/gateway && go fmt ./... && \
	cd ../agents && ruff format . && \
	cd ../web && npm run format --silent
