package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// SetupTailscale configures tailscale serve/funnel for the share server.
func SetupTailscale(shareAddr string) (string, error) {
	binary, err := detectTailscale()
	if err != nil {
		return "", err
	}
	port, err := sharePort(shareAddr)
	if err != nil {
		return "", err
	}

	if err := runTailscale(binary, "serve", "--bg", "--yes", "--http", port); err != nil {
		return "", err
	}
	if err := runTailscale(binary, "funnel", "--bg", "--yes", port); err != nil {
		return "", err
	}

	host, err := tailscaleHost(binary)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(host, "."), nil
}

type tailscaleStatus struct {
	Self struct {
		DNSName string `json:"DNSName"`
	} `json:"Self"`
}

func detectTailscale() (string, error) {
	macPath := "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
	if info, err := os.Stat(macPath); err == nil && !info.IsDir() {
		return macPath, nil
	}
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return "", errors.New("tailscale binary not found")
	}
	return path, nil
}

func sharePort(shareAddr string) (string, error) {
	if shareAddr == "" {
		return "", errors.New("share-addr is empty")
	}
	port := ""
	if strings.HasPrefix(shareAddr, ":") {
		port = strings.TrimPrefix(shareAddr, ":")
	} else if strings.Contains(shareAddr, ":") {
		_, p, err := net.SplitHostPort(shareAddr)
		if err != nil {
			return "", err
		}
		port = p
	} else {
		port = shareAddr
	}
	if port == "" {
		return "", errors.New("share-addr missing port")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("share-addr port is not numeric")
	}
	return port, nil
}

func runTailscale(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func tailscaleHost(binary string) (string, error) {
	cmd := exec.Command(binary, "status", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return "", err
	}
	if status.Self.DNSName == "" {
		return "", errors.New("tailscale status missing DNSName")
	}
	return status.Self.DNSName, nil
}
