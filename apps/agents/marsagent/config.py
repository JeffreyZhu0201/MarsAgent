"""加载所有环境变量；通过 Settings() 单例使用。"""
import threading

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # HTTP（FastAPI，仅用于 /healthz）
    agents_http_port: int = 8001
    # gRPC server
    agents_grpc_port: int = 50051
    # Redis
    redis_url: str = "redis://localhost:6379/0"
    # 任务消费配置
    stream_group: str = "agents-workers"
    stream_consumer: str = "worker-1"
    server_version: str = "m1-dev"


_settings: Settings | None = None
_settings_lock = threading.Lock()


def get_settings() -> Settings:
    global _settings
    if _settings is None:
        with _settings_lock:
            if _settings is None:
                _settings = Settings()
    return _settings
