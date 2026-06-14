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
	Host                   string        `yaml:"-"`
	Port                   int           `yaml:"-"`
	LogLevel               string        `yaml:"log_level"`
	AccrualIntervalSeconds int           `yaml:"accrual_interval_seconds"`
	Workers                int           `yaml:"worker_concurrency"`
	DatabaseTimeout        time.Duration `yaml:"database_timeout"`
	DatabaseURI            string        `yaml:"-"`
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
	}

	data, err := os.ReadFile("./config.yaml")
	if err != nil {
		return config, nil
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка десериализации конфигурации: %w", err)
	}
	var (
		dbHost   = "localhost"
		dbName   = "gopherledger"
		login    = "user"
		password = "password"
	)

	if envHost := os.Getenv("DB_HOST"); envHost != "" {
		dbHost = envHost
	}
	if envName := os.Getenv("DB_NAME"); envName != "" {
		dbName = envName
	}
	if envLogin := os.Getenv("DB_LOGIN"); envLogin != "" {
		login = envLogin
	}
	if envPassword := os.Getenv("DB_PASSWORD"); envPassword != "" {
		password = envPassword
	}

	config.DatabaseURI = fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable",
		login, password, dbHost, dbName,
	)
	return config, nil
}
