package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	MaxLobbySize int
	QuestionTime int // seconds
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	databaseURL := getEnv("DATABASE_URL", "postgres://quizuser:quizpass@localhost:5432/quizdb?sslmode=disable")
	maxLobbySize := getEnvAsInt("MAX_LOBBY_SIZE", 8)
	questionTime := getEnvAsInt("QUESTION_TIME", 30)

	return &Config{
		Port:         port,
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
