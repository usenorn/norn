package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

const dotenvFile = ".env"

func loadDotenv() error {
	file, err := os.Open(dotenvFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}

		key = strings.TrimSpace(key)

		if _, set := os.LookupEnv(key); set {
			continue
		}

		if err := os.Setenv(key, strings.TrimSpace(value)); err != nil {
			return err
		}
	}

	return scanner.Err()
}
