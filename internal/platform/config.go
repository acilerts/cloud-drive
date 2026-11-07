package platform

import (
	"fmt"
	"os"
)

func Required(key string) (string, error) {
	value := os.Getenv(key)

	if value == "" {
		return "", fmt.Errorf("missing required enviroment variable %s", key)
	}

	return value, nil
}

func Default(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
