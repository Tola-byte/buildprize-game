package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	RedisURL     string
	DatabaseURL  string
	MaxLobbySize int
	QuestionTime int // seconds
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	databaseURL := getEnv("DATABASE_URL", "")
	maxLobbySize := getEnvAsInt("MAX_LOBBY_SIZE", 8)
	questionTime := getEnvAsInt("QUESTION_TIME", 30)

	return &Config{
		Port:         port,
		RedisURL:     redisURL,
		DatabaseURL:  databaseURL,
		MaxLobbySize: maxLobbySize,
		QuestionTime: questionTime,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
