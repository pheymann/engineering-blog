// vault-preview-service is the long-running local vault-to-HTML preview pipeline.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pheymann/engineering-blog/internal/config"
	"github.com/pheymann/engineering-blog/internal/previewbuilder"
	"github.com/pheymann/engineering-blog/internal/watchservice"
)

func main() {
	buildOnce := flag.Bool("build-once", false, "build one validated preview snapshot and exit")
	flag.Parse()
	if err := run(*buildOnce); err != nil {
		fmt.Fprintf(os.Stderr, "vault-preview-service: %v\n", err)
		os.Exit(1)
	}
}

func run(buildOnce bool) error {
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	context, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("vault-preview-service started: vault=%s output=%s server=%s debounce=%s reconciliation=%s\n",
		configuration.VaultDirectory,
		configuration.PreviewOutputDirectory,
		configuration.PreviewServerURL,
		configuration.DebounceInterval,
		configuration.ReconciliationInterval,
	)

	staticDirectory, err := filepath.Abs("src/site")
	if err != nil {
		return fmt.Errorf("resolve site source: %w", err)
	}
	builder := previewbuilder.Config{
		VaultDirectory:  configuration.VaultDirectory,
		StaticDirectory: staticDirectory,
		OutputDirectory: configuration.PreviewOutputDirectory,
	}
	if buildOnce {
		result, err := previewbuilder.Build(builder)
		if err != nil {
			return fmt.Errorf("build preview: %w", err)
		}
		fmt.Printf("vault-preview-service preview build finished (changed=%t posts=%d)\n", result.Changed, result.Posts)
		return nil
	}
	if err := watchservice.Run(context, watchservice.Config{
		VaultDirectory:         configuration.VaultDirectory,
		DebounceInterval:       configuration.DebounceInterval,
		ReconciliationInterval: configuration.ReconciliationInterval,
		Build: func() error {
			_, err := previewbuilder.Build(builder)
			return err
		},
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "vault-preview-service stopped")
	return nil
}
