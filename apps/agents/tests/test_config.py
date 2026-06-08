import os

from marsagent.config import Settings


def test_settings_defaults():
    s = Settings(_env_file=None)
    assert s.agents_http_port == 8001
    assert s.agents_grpc_port == 50051
    assert s.redis_url.startswith("redis://")


def test_settings_env_override(monkeypatch):
    monkeypatch.setenv("AGENTS_HTTP_PORT", "9999")
    monkeypatch.setenv("REDIS_URL", "redis://1.2.3.4:6379/2")
    s = Settings(_env_file=None)
    assert s.agents_http_port == 9999
    assert s.redis_url == "redis://1.2.3.4:6379/2"
