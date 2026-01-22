package app

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readEnvFile(path string) map[string]string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return map[string]string{}
	}

	file, err := os.Open(path)
	if err != nil {
		return map[string]string{}
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			if parsed, err := strconv.Unquote(value); err == nil {
				value = parsed
			}
		}
		env[key] = value
	}

	return env
}
