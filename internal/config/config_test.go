package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFromAcceptsValidConfiguration(t *testing.T) {
	vault := t.TempDir()
	outputParent := t.TempDir()
	output := filepath.Join(outputParent, "site")
	values := map[string]string{
		vaultDirectoryEnv:         vault,
		previewOutputDirectoryEnv: output,
		previewServerURLEnv:       "http://127.0.0.1:8081",
		debounceIntervalEnv:       "250ms",
		reconciliationIntervalEnv: "2m",
	}

	configuration, err := LoadFrom(valuesLookup(values))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if configuration.VaultDirectory != vault || configuration.PreviewOutputDirectory != output {
		t.Fatalf("unexpected paths: %#v", configuration)
	}
	if configuration.PreviewServerURL.String() != "http://127.0.0.1:8081" {
		t.Fatalf("PreviewServerURL = %q", configuration.PreviewServerURL)
	}
	if configuration.DebounceInterval != 250*time.Millisecond || configuration.ReconciliationInterval != 2*time.Minute {
		t.Fatalf("unexpected durations: %#v", configuration)
	}
}

func TestLoadFromRequiresVaultDirectory(t *testing.T) {
	_, err := LoadFrom(valuesLookup(map[string]string{previewOutputDirectoryEnv: t.TempDir()}))
	if err == nil || !strings.Contains(err.Error(), vaultDirectoryEnv+" must be set") {
		t.Fatalf("LoadFrom() error = %v, want missing vault-directory error", err)
	}
}

func TestLoadFromRejectsOutputInsideVault(t *testing.T) {
	vault := t.TempDir()
	_, err := LoadFrom(valuesLookup(map[string]string{
		vaultDirectoryEnv:         vault,
		previewOutputDirectoryEnv: filepath.Join(vault, "generated"),
	}))
	if err == nil || !strings.Contains(err.Error(), "must not be inside") {
		t.Fatalf("LoadFrom() error = %v, want output-loop error", err)
	}
}

func TestLoadFromRejectsVaultInsideOutput(t *testing.T) {
	output := t.TempDir()
	vault := filepath.Join(output, "vault")
	if err := os.Mkdir(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(valuesLookup(map[string]string{
		vaultDirectoryEnv:         vault,
		previewOutputDirectoryEnv: output,
	}))
	if err == nil || !strings.Contains(err.Error(), "must not be inside") {
		t.Fatalf("LoadFrom() error = %v, want output-loop error", err)
	}
}

func TestLoadFromRejectsOutputThatReachesVaultThroughSymlink(t *testing.T) {
	vault := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "vault-link")
	if err := os.Symlink(vault, link); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(valuesLookup(map[string]string{
		vaultDirectoryEnv:         vault,
		previewOutputDirectoryEnv: filepath.Join(link, "generated"),
	}))
	if err == nil || !strings.Contains(err.Error(), "must not be inside") {
		t.Fatalf("LoadFrom() error = %v, want output-loop error", err)
	}
}

func valuesLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
