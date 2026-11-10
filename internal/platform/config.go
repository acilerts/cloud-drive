package platform

import (
	"fmt"
	"os"
	"strconv"
	"time"
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

func Duration(key, fallback string) (time.Duration, error) {
	d, err := time.ParseDuration(Default(key, fallback))
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration in %s", key)
	}

	return d, nil
}

func Int(key string, fallback int) (int, error) {
	n, err := strconv.Atoi(Default(key, strconv.Itoa(fallback)))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number in %s", key)
	}

	return n, nil
}
