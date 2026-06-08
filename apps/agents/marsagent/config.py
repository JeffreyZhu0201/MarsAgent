"""加载所有环境变量；通过 Settings() 单例使用。"""
import threading

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        extra="ignore",
        protected_namespaces=("settings_",),
    )

    # HTTP（FastAPI，仅用于 /healthz）
    agents_http_port: int = 8001
    # gRPC server
    agents_grpc_port: int = 50051
    # Redis
    redis_url: str = "redis://localhost:6379/0"
    # 任务消费配置
    stream_group: str = "agents-workers"
    stream_consumer: str = "worker-1"
    stream_max_attempts: int = 3
    stream_retry_delay_ms: int = 1000
    stream_dlq_suffix: str = ":dlq"
    server_version: str = "m1-dev"
    # Anthropic-compatible LLM endpoint/model routing.
    llm_api_key: str = ""
    llm_base_url: str = ""
    model_haiku: str = "claude-haiku-4-5-20251001"
    model_sonnet: str = "claude-sonnet-4-6"
    model_opus: str = "claude-opus-4-8"


_settings: Settings | None = None
_settings_lock = threading.Lock()


def get_settings() -> Settings:
    global _settings
    if _settings is None:
        with _settings_lock:
            if _settings is None:
                _settings = Settings()
    return _settings
