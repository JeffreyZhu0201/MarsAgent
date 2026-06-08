package config

import (
	"github.com/caarlos0/env/v11"
)

// Config 是 gateway 的全部运行时配置；通过环境变量注入。
// 命名遵守 GATEWAY_* / 后续可加前缀分组。
type Config struct {
	HTTPPort       int    `env:"GATEWAY_HTTP_PORT" envDefault:"8080"`
	RedisURL       string `env:"REDIS_URL"          envDefault:"redis://localhost:6379/0"`
	GRPCTarget     string `env:"GATEWAY_GRPC_TARGET" envDefault:"localhost:50051"`
	ServerVersion  string `env:"SERVER_VERSION"     envDefault:"m1-dev"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, err
	}
	return c, nil
}