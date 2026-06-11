"""持久化容器池 — 每种语言维持一个预热容器。

通过 container.exec_run() 在运行中的容器内执行用户代码，
避免每次请求都启动/销毁容器。
"""
from __future__ import annotations

import os
import time
from dataclasses import dataclass
from typing import Literal

import docker
from docker.models.containers import Container

# 语言 → Docker 镜像映射
IMAGE_MAP: dict[str, str] = {
    "python": "marsagent-python:WithLibs",
    "node": "node:20-alpine",
    "go": "golang:1.22-alpine",
}

MAX_CODE_BYTES = 64 * 1024
MAX_OUTPUT_BYTES = 64 * 1024
MAX_TIMEOUT_SEC = 30


@dataclass
class RunResult:
    """沙箱单次执行结果。"""

    stdout: str
    stderr: str
    exit_code: int
    duration_ms: int
    truncated: bool


def detect_docker_host() -> str:
    """检测 Docker socket 地址，兼容 OrbStack（macOS）。"""
    if host := os.environ.get("DOCKER_HOST", ""):
        return host
    user = os.environ.get("USER", "")
    if user:
        orb = f"/Users/{user}/.orbstack/run/docker.sock"
        if os.path.exists(orb):
            return f"unix://{orb}"
    return "unix:///var/run/docker.sock"


class ContainerPool:
    """每种语言维护一个长期运行的 Docker 容器。"""

    def __init__(self) -> None:
        try:
            self._client = docker.DockerClient(base_url=detect_docker_host())
            self._client.ping()
        except Exception as exc:
            print(f"[ContainerPool] Docker 不可用: {exc}")
            self._client = None
        self._containers: dict[str, Container] = {}

    def ensure_started(self) -> None:
        """启动全部语言容器（进程启动时调用一次）。"""
        if self._client is None:
            return
        for lang, image in IMAGE_MAP.items():
            if lang in self._containers:
                try:
                    self._containers[lang].reload()
                    continue
                except Exception:
                    pass
            try:
                self._containers[lang] = self._start(lang, image)
            except Exception as exc:
                print(f"[ContainerPool] 启动 {lang} 失败: {exc}")

    def _start(self, lang: str, image: str) -> Container:
        """创建并启动指定语言的容器。"""
        name = f"marsagent-{lang}"
        try:
            stale = self._client.containers.get(name)
            stale.stop(timeout=1)
            stale.remove(force=True)
        except Exception:
            pass

        c = self._client.containers.run(
            image,
            "tail -f /dev/null",
            name=name,
            detach=True,
            mem_limit="256m",
            cpu_period=100000,
            cpu_quota=50000,
            network_mode="none",
            security_opt=["no-new-privileges"],
            auto_remove=False,
        )
        print(f"[ContainerPool] 已启动 {lang} 容器 {c.short_id}")
        return c

    def run(
        self,
        code: str,
        lang: Literal["python", "node", "go"],
        stdin: str = "",
        timeout_sec: int = 15,
    ) -> RunResult:
        """在预热容器中执行代码。"""
        if lang not in IMAGE_MAP:
            return RunResult("", f"unsupported language: {lang}", 1, 0, False)
        if len(code) > MAX_CODE_BYTES:
            return RunResult("", f"code exceeds {MAX_CODE_BYTES} bytes", 1, 0, False)
        if self._client is None:
            return RunResult("", "docker not available", 1, 0, False)

        container = self._containers.get(lang)
        if container is None:
            return RunResult("", f"container for {lang} not running", 1, 0, False)

        timeout_sec = min(timeout_sec, MAX_TIMEOUT_SEC)
        t0 = time.monotonic()

        try:
            resp = container.exec_run(
                self._build_cmd(lang, code, stdin),
                stdout=True,
                stderr=True,
                socket=False,
            )
        except Exception as exc:
            duration_ms = int((time.monotonic() - t0) * 1000)
            return RunResult("", str(exc), 1, duration_ms, False)

        duration_ms = int((time.monotonic() - t0) * 1000)
        output = resp.output.decode("utf-8", errors="replace")
        truncated = len(resp.output) > MAX_OUTPUT_BYTES
        if truncated:
            output = output[:MAX_OUTPUT_BYTES]

        return RunResult(output, "", resp.exit_code, duration_ms, truncated)

    def _build_cmd(self, lang: Literal["python", "node", "go"], code: str, stdin: str) -> list[str]:
        """构造容器内执行命令。"""
        if lang == "python":
            return ["/bin/sh", "-c", f"python3 -c {self._shell_escape(code)}"]
        if lang == "node":
            return ["/bin/sh", "-c", f"node -e {self._shell_escape(code)}"]
        # Go：写入 /tmp/main.go 后 go run
        stdin_part = f" <<EOF\n{stdin}\nEOF" if stdin else ""
        return [
            "/bin/sh",
            "-c",
            f"cat > /tmp/main.go << 'GOEOF'\n{code}\nGOEOF\n"
            f"cd /tmp && go run main.go{stdin_part}",
        ]

    @staticmethod
    def _shell_escape(s: str) -> str:
        """单引号 shell 转义。"""
        return "'" + s.replace("'", "'\"'\"'") + "'"

    def close(self) -> None:
        """停止并移除全部容器。"""
        for lang, c in list(self._containers.items()):
            try:
                c.stop(timeout=1)
                c.remove(force=True)
                print(f"[ContainerPool] 已停止 {lang} ({c.short_id})")
            except Exception as exc:
                print(f"[ContainerPool] 停止 {lang} 出错: {exc}")
        self._containers.clear()
