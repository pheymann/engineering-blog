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
	"github.com/pheymann/engineering-blog/internal/gitpublisher"
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

	fmt.Printf("vault-preview-service started: vault=%s preview=%s production=%s server=%s debounce=%s reconciliation=%s\n",
		configuration.VaultDirectory,
		configuration.PreviewOutputDirectory,
		configuration.ProductionOutputDirectory,
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
	productionBuilder := previewbuilder.Config{
		VaultDirectory: configuration.VaultDirectory, StaticDirectory: staticDirectory,
		OutputDirectory: configuration.ProductionOutputDirectory,
		StatePath:       configuration.ProductionStatePath,
		DeploymentTag:   "publish", PreserveRemoved: true,
	}
	build := func() error {
		if _, err := previewbuilder.Build(builder); err != nil {
			return fmt.Errorf("build preview: %w", err)
		}
		_, err := previewbuilder.Build(productionBuilder)
		if err != nil {
			return fmt.Errorf("build production: %w", err)
		}
		published, err := gitpublisher.Publish(gitpublisher.Config{RepositoryDirectory: configuration.GitRepositoryDirectory, OutputDirectory: configuration.ProductionOutputDirectory})
		if err != nil {
			return fmt.Errorf("publish production output: %w", err)
		}
		if published {
			fmt.Println("vault-preview-service production output published")
		}
		return nil
	}
	if buildOnce {
		if err := build(); err != nil {
			return err
		}
		fmt.Println("vault-preview-service preview and production build finished")
		return nil
	}
	if err := watchservice.Run(context, watchservice.Config{
		VaultDirectory:         configuration.VaultDirectory,
		DebounceInterval:       configuration.DebounceInterval,
		ReconciliationInterval: configuration.ReconciliationInterval,
		Build: func() error {
			return build()
		},
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "vault-preview-service stopped")
	return nil
}
