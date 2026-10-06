package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Log             Log
	Ca              CaConfig
	DBConfig        PostgresConfig
	ServerAddress   ServerAddress
	Storage         StorageConfig
	Target          TargetConfig
	AnomalyDetector AnomalyDetectorConfig
	DownloadConfig  DownloadConfig
}

type Log struct {
	//DEBUG/INFO/WARN/ERROR
	Level     string `env:"LOG_LEVEL" env-default:"INFO"`
	AddSource bool   `env:"LOG_SOURCE" env-default:"false"`
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

	// Time interval when run storage cleaner / Time to Start Storage Cleanup
	// Valid time units "s", "m", "h".
	CleanupInterval time.Duration `env:"STORAGE_CLEANUP_INTERVAL" env-default:"1h"`

	// The maximum amount of time a file should be held in cache.
	// 1m - a month 2w - 2 weeks, 10d - 10 days
	// Valid time units "d", "w", "m".
	FileTTL string `env:"STORAGE_FILE_TTL" env-default:"2w"`

	// The amount of disk space that a container can use for data caching. Set a maximum disk space limit to 80%.
	// 1T - 1TB, 100G - 100GB, 10000M - 10000MB
	// Valid time units "M", "G", "T".
	DiskSize string `env:"STORAGE_DISK_SIZE" env-default:"512G"`

	// Set Storage cache ttl in minutes
	CacheTTL int `env:"STORAGE_CACHE_TTL" env-default:"5"`
}
type AnomalyDetectorConfig struct {
	// Detect history TTL in second
	HistoryTTL int `env:"ANOMALY_DETECTOR_TTL" env-default:"15"`

	// in second
	CleanupTimeTicker int `env:"ANOMALY_DETECTOR_CLEANUP_TIME_TICKER" env-default:"15"`

	// if a history > HistoryMax at HistoryTTL it's anomaly
	HistoryMax int `env:"ANOMALY_DETECTOR_MAX" env-default:"3"`
}
type DownloadConfig struct {
	NumberOfWorkers int `env:"DOWNLOAD_NUMBER_OF_WORKERS" env-default:"10"`
}

type TargetConfig struct {
	// The github repo url for the cache_domains files to use to populate the cdn maps,
	// if you do not want to use the recommended source for testing purposes.
	CacheDomainsUrl string `env:"CACHE_DOMAINS_URL"`
}

type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST" env-default:"postgres"`
	Port     string `env:"POSTGRES_PORT" env-default:"5432"`
	User     string `env:"POSTGRES_USER" env-default:"lancache"`
	Password string `env:"POSTGRES_PASSWORD" env-default:"lancache"`
	DB       string `env:"POSTGRES_DB" env-default:"lancache"`
}

func Load() Config {
	var cfg Config

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file - " + err.Error())
	}

	err = cleanenv.ReadEnv(&cfg)
	if err != nil {
		panic("cannot init config: " + err.Error())
	}

	return cfg
}
