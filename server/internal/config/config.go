package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisURL string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return Config{}, errors.New("REDIS_URL not set")
	}

	return Config{RedisURL: redisURL}, nil
}
