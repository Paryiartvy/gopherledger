// Пакет config загружает конфигурацию приложения из YAML-файла.
// Реализуйте этот пакет самостоятельно.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config содержит параметры запуска сервера.
// Изучите config.yaml и добавьте поля самостоятельно.
type Config struct {
	Host                   string        `yaml:"server_host"`
	Port                   int           `yaml:"server_port"`
	LogLevel               string        `yaml:"log_level"`
	AccrualIntervalSeconds int           `yaml:"accrual_interval_seconds"`
	Workers                int           `yaml:"worker_concurrency"`
	DatabaseTimeout        time.Duration `yaml:"database_timeout"`
	DatabaseURI            string        `yaml:"database_uri"`
}

var GlobalConfig *Config

// Load читает конфигурацию из файла config.yaml.
// Если файл не найден или поле не задано, применяются значения по умолчанию.
func Load() (*Config, error) {
	config := &Config{
		LogLevel:               "info",
		AccrualIntervalSeconds: 3,
		Workers:                5,
		DatabaseTimeout:        5 * time.Second,
		DatabaseURI:            "postgres://user:password@localhost:5432/gopherledger?sslmode=disable",
	}

	data, err := os.ReadFile("./config.yaml")
	if err != nil {
		return config, nil
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка десериализации конфигурации: %w", err)
	}
	return config, nil
}
