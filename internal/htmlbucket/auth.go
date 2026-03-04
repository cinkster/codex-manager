package htmlbucket

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultAuthDir  = ".hb"
	defaultAuthFile = "auth.json"
)

// Auth is the htmlbucket local auth file format.
type Auth struct {
	APIKey string `json:"api_key"`
}

// DefaultAuthPath returns the default auth file path ($HOME/.hb/auth.json).
func DefaultAuthPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultAuthDir, defaultAuthFile), nil
}

// LoadAuth reads and validates auth JSON.
func LoadAuth(path string) (Auth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Auth{}, err
	}
	var auth Auth
	if err := json.Unmarshal(data, &auth); err != nil {
		return Auth{}, err
	}
	auth.APIKey = strings.TrimSpace(auth.APIKey)
	if auth.APIKey == "" {
		return Auth{}, errors.New("api_key is empty")
	}
	return auth, nil
}

// WriteAuth writes auth JSON and enforces restrictive permissions.
func WriteAuth(path string, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("api key is empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(Auth{APIKey: apiKey}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// PromptAPIKey prompts once for an API key and validates non-empty input.
func PromptAPIKey(in io.Reader, out io.Writer) (string, error) {
	if in == nil {
		return "", errors.New("input reader is nil")
	}
	if out != nil {
		if _, err := fmt.Fprint(out, "Enter htmlbucket API key: "); err != nil {
			return "", err
		}
	}

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no input received")
	}
	apiKey := strings.TrimSpace(scanner.Text())
	if apiKey == "" {
		return "", errors.New("api key is empty")
	}
	return apiKey, nil
}
