"""ContainerPool 单元测试 — 不依赖真实 Docker。"""
from marsagent.sandbox.pool import (
    MAX_CODE_BYTES,
    ContainerPool,
    detect_docker_host,
)


def test_detect_docker_host_default():
    host = detect_docker_host()
    assert host.startswith("unix://") or host.startswith("tcp://") or host.startswith("http")


def test_shell_escape():
    assert ContainerPool._shell_escape("hello") == "'hello'"
    assert ContainerPool._shell_escape("it's") == "'it'\"'\"'s'"


def test_build_cmd_python():
    pool = ContainerPool()
    cmd = pool._build_cmd("python", 'print("hi")', "")
    assert cmd[0] == "/bin/sh"
    assert "python3 -c" in cmd[2]
    assert "print" in cmd[2]


def test_build_cmd_go_with_stdin():
    pool = ContainerPool()
    cmd = pool._build_cmd("go", 'package main\nfunc main(){}', "input")
    assert "main.go" in cmd[2]
    assert "EOF" in cmd[2]


def test_run_unsupported_language():
    pool = ContainerPool()
    pool._client = None  # 跳过 Docker 连接
    result = pool.run(code="x", lang="rust")  # type: ignore[arg-type]
    assert result.exit_code == 1
    assert "unsupported" in result.stderr


def test_run_code_too_large():
    pool = ContainerPool()
    big = "x" * (MAX_CODE_BYTES + 1)
    result = pool.run(code=big, lang="python")
    assert result.exit_code == 1
    assert "exceeds" in result.stderr


def test_run_docker_unavailable():
    pool = ContainerPool()
    pool._client = None
    result = pool.run(code="print(1)", lang="python")
    assert result.exit_code == 1
    assert "docker not available" in result.stderr
