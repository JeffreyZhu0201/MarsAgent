# syntax=docker/dockerfile:1.7
FROM python:3.11-slim AS base
ENV PYTHONUNBUFFERED=1 PIP_NO_CACHE_DIR=1
WORKDIR /app
COPY pyproject.toml ./
RUN pip install -U pip && pip install .

COPY . .
# 容器内重新生成 proto（保持单源真相）
RUN apt-get update && apt-get install -y --no-install-recommends protobuf-compiler && rm -rf /var/lib/apt/lists/*
RUN python -m grpc_tools.protoc \
    --proto_path=/proto \
    --python_out=marsagent/gen \
    --grpc_python_out=marsagent/gen \
    /proto/wiki.proto || true
# 注意：实际 docker compose 会把 /proto 挂进来；此处 || true 兼容单独 build

EXPOSE 8001 50051
CMD ["uvicorn", "marsagent.main:app", "--host", "0.0.0.0", "--port", "8001"]
