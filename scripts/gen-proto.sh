#!/usr/bin/env bash
# 生成 Go + Python 的 gRPC 代码到各自的 gen/ 目录
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT/proto"

GO_OUT="$ROOT/apps/gateway/gen/proto"
PY_OUT="$ROOT/apps/agents/marsagent/gen"

mkdir -p "$GO_OUT" "$PY_OUT"

echo "==> Generating Go code"
protoc \
  --proto_path="$PROTO_DIR" \
  --go_out="$GO_OUT" --go_opt=paths=source_relative \
  --go-grpc_out="$GO_OUT" --go-grpc_opt=paths=source_relative \
  "$PROTO_DIR/wiki.proto"

echo "==> Generating Python code"
python -m grpc_tools.protoc \
  --proto_path="$PROTO_DIR" \
  --python_out="$PY_OUT" \
  --grpc_python_out="$PY_OUT" \
  "$PROTO_DIR/wiki.proto"

# Python 生成的 import 是 `import wiki_pb2`，包内导入需要修补为相对导入。
python - <<PY
from pathlib import Path
p = Path(r"$PY_OUT") / "wiki_pb2_grpc.py"
p.write_text(p.read_text().replace("import wiki_pb2 as wiki__pb2", "from . import wiki_pb2 as wiki__pb2"))
PY
touch "$PY_OUT/__init__.py"

echo "==> Done"
