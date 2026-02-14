package app

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string `env:"TELEGRAM_TOKEN,required"`
	EthWSURL      string `env:"ETH_WS_URL,required"`
	PostgresURL   string `env:"POSTGRES_URL,required"`

	WatcherWorkers int `env:"WATCHER_WORKERS"`
	TasksBuffer    int `env:"TASKS_BUFFER"`
	NotifyBuffer   int `env:"NOTIFY_BUFFER"`
}

func LoadConfig() (Config, error) {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: .env file not found, relying on environment variables")
	}

	config := Config{
		WatcherWorkers: 8,
		TasksBuffer:    4096,
		NotifyBuffer:   4096,
	}

	if err := env.Parse(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}
