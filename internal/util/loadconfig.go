package util

import (
	"os"

	"github.com/joho/godotenv"
)

func LoadConfig() Config {
	godotenv.Load()

	return Config{
		ENV:   fallback(os.Getenv("ENV"), "development"),
		CRT:   os.Getenv("CRT"),
		KEY:   os.Getenv("KEY"),
		PORT:  fallback(os.Getenv("PORT"), "5098"),
		LINKS: os.Getenv("LINKS"),
	}
}

type Config struct {
	ENV   string
	CRT   string
	KEY   string
	PORT  string
	LINKS string
}

func fallback(value, def string) string {
	if value == "" {
		return def
	}
	return value
}
