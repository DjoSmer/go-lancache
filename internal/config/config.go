package config

import "github.com/ilyakaznacheev/cleanenv"

type Config struct {
	LogLevel string `env:"LOG_LEVEL" env-default:"warning"`
	// The github repo url for the cache_domains files to use to populate the cdn maps, if you do not want to use the recommended source for testing purposes.
	UrlCacheDomains string `env:"URL_CACHE_DOMAINS"`

	Ca            CaConfig
	DBConfig      PostgresConfig
	ServerAddress ServerAddress
	Storage       StorageConfig
}

type ServerAddress struct {
	Http  string `env:"HTTP_ADDR" env-default:":80"`
	Https string `env:"HTTPS_ADDR" env-default:":443"`
}

type CaConfig struct {
	Cert string `env:"CA_CERT" env-default:"cert/ca.pem"`
	Key  string `env:"CA_KEY" env-default:"cert/key.pem"`
}

type StorageConfig struct {
	// Dir for target files
	Path string `env:"STORAGE_PATH"`

	// The maximum amount of time a file should be held in cache.
	// 1m - a month 2w - 2 weeks, 10d - 10 days
	MaxAge string `env:"STORAGE_MAX_AGE" env-default:"2w"`

	// The amount of disk space the container should use for caching data. Specified in gigabytes.
	// 1T - 1TB, 100G - 100GB, 10000M - 10000MB
	DiskSize string `env:"STORAGE_DISK_SIZE" env-default:"512G"`
}

type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST" env-default:"postgres"`
	Port     string `env:"POSTGRES_PORT" env-default:"5432"`
	User     string `env:"POSTGRES_USER" env-default:"lancache"`
	Password string `env:"POSTGRES_PASSWORD" env-default:"lancache"`
	DB       string `env:"POSTGRES_DB" env-default:"lancache"`
}

func Load() *Config {
	var cfg Config

	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		panic("cannot init config: " + err.Error())
	}
	return &cfg
}
