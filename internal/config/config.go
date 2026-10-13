package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	LogLevel     string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		KafkaBrokers: splitCSV(env("KAFKA_BROKERS", "localhost:29092")),
		KafkaTopic:   env("KAFKA_TOPIC", "phonebook.contacts"),
		KafkaGroupID: env("KAFKA_GROUP_ID", "phonebook-audit"),
		LogLevel:     env("LOG_LEVEL", "info"),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS must not be empty")
	}
	return c, nil
}

func (c Config) NewLogger() *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
