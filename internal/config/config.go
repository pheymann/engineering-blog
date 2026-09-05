// Package config loads the settings shared by the vault preview service.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	vaultDirectoryEnv             = "BLOG_VAULT_DIRECTORY"
	previewOutputDirectoryEnv     = "BLOG_PREVIEW_OUTPUT_DIRECTORY"
	previewServerURLEnv           = "BLOG_PREVIEW_SERVER_URL"
	debounceIntervalEnv           = "BLOG_DEBOUNCE_INTERVAL"
	reconciliationIntervalEnv     = "BLOG_RECONCILIATION_INTERVAL"
	defaultPreviewServerURL       = "http://127.0.0.1:8080"
	defaultDebounceInterval       = 500 * time.Millisecond
	defaultReconciliationInterval = 5 * time.Minute
)

// Config contains the filesystem and timing settings used by the preview pipeline.
// The service owns neither the HTTP server nor the generated output format.
type Config struct {
	VaultDirectory         string
	PreviewOutputDirectory string
	PreviewServerURL       *url.URL
	DebounceInterval       time.Duration
	ReconciliationInterval time.Duration
}

// Load reads configuration from the process environment and validates it before
// any service work starts.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom is equivalent to Load but accepts an environment lookup function for tests.
func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	vaultDirectory, err := requiredDirectory(lookup, vaultDirectoryEnv)
	if err != nil {
		return Config{}, err
	}

	previewOutputDirectory, err := requiredPath(lookup, previewOutputDirectoryEnv)
	if err != nil {
		return Config{}, err
	}

	if pathsOverlap(vaultDirectory, previewOutputDirectory) {
		return Config{}, fmt.Errorf("%s (%q) must not be inside %s (%q), and %s must not be inside it", previewOutputDirectoryEnv, previewOutputDirectory, vaultDirectoryEnv, vaultDirectory, vaultDirectoryEnv)
	}

	previewServerURL, err := parsePreviewServerURL(valueOrDefault(lookup, previewServerURLEnv, defaultPreviewServerURL))
	if err != nil {
		return Config{}, err
	}

	debounceInterval, err := durationOrDefault(lookup, debounceIntervalEnv, defaultDebounceInterval)
	if err != nil {
		return Config{}, err
	}
	reconciliationInterval, err := durationOrDefault(lookup, reconciliationIntervalEnv, defaultReconciliationInterval)
	if err != nil {
		return Config{}, err
	}

	return Config{
		VaultDirectory:         vaultDirectory,
		PreviewOutputDirectory: previewOutputDirectory,
		PreviewServerURL:       previewServerURL,
		DebounceInterval:       debounceInterval,
		ReconciliationInterval: reconciliationInterval,
	}, nil
}

func requiredDirectory(lookup func(string) (string, bool), name string) (string, error) {
	path, err := requiredPath(lookup, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s (%q) is not accessible: %w", name, path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s (%q) must be a directory", name, path)
	}
	return path, nil
}

func requiredPath(lookup func(string) (string, bool), name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must be set", name)
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	path, err = canonicalPath(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	return path, nil
}

// canonicalPath resolves symlinks in the existing portion of a path. Output
// directories usually do not exist yet, so EvalSymlinks alone is insufficient.
func canonicalPath(path string) (string, error) {
	missing := make([]string, 0)
	for {
		info, err := os.Lstat(path)
		if err == nil {
			if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 && len(missing) > 0 {
				return "", fmt.Errorf("%q is not a directory", path)
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

func valueOrDefault(lookup func(string) (string, bool), name, fallback string) string {
	if value, ok := lookup(name); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func durationOrDefault(lookup func(string) (string, bool), name string, fallback time.Duration) (time.Duration, error) {
	value := valueOrDefault(lookup, name, fallback.String())
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration (for example %q)", name, fallback)
	}
	return duration, nil
}

func parsePreviewServerURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute http URL", previewServerURLEnv)
	}
	return parsed, nil
}

func pathsOverlap(first, second string) bool {
	return first == second || isWithin(first, second) || isWithin(second, first)
}

func isWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
