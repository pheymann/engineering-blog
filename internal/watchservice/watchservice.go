// Package watchservice watches a vault and serializes preview reconciliations.
package watchservice

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config describes one vault watcher. Build must validate and atomically commit
// a complete preview snapshot; it is never called concurrently.
type Config struct {
	VaultDirectory         string
	DebounceInterval       time.Duration
	ReconciliationInterval time.Duration
	Build                  func() error
	Logf                   func(string, ...any)
}

// Run performs a startup reconciliation, watches every existing and newly
// created vault directory, and periodically reconciles until ctx is cancelled.
func Run(ctx context.Context, config Config) error {
	if err := validate(config); err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create vault watcher: %w", err)
	}
	defer watcher.Close()
	if err := addTree(watcher, config.VaultDirectory); err != nil {
		return fmt.Errorf("watch vault directories: %w", err)
	}

	logf := config.Logf
	if logf == nil {
		logf = log.Printf
	}
	ticker := time.NewTicker(config.ReconciliationInterval)
	defer ticker.Stop()

	type buildResult struct{ err error }
	completed := make(chan buildResult, 1)
	pending := make(map[string]struct{})
	needsReconciliation := true // startup detects changes made while stopped.
	building := false
	var debounce <-chan time.Time
	var timer *time.Timer

	startBuild := func(reason string) {
		building = true
		paths := sortedPaths(pending)
		pending = make(map[string]struct{})
		needsReconciliation = false
		logf("preview build started (%s; %d changed paths)", reason, len(paths))
		go func() { completed <- buildResult{err: config.Build()} }()
	}
	schedule := func() {
		if building || len(pending) == 0 || timer != nil {
			return
		}
		timer = time.NewTimer(config.DebounceInterval)
		debounce = timer.C
	}
	resetDebounce := func() {
		if building {
			return
		}
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = time.NewTimer(config.DebounceInterval)
		debounce = timer.C
	}

	for {
		if !building && needsReconciliation {
			startBuild("reconciliation")
			continue
		}
		schedule()
		select {
		case <-ctx.Done():
			if timer != nil && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("vault watcher events closed")
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			pending[event.Name] = struct{}{}
			resetDebounce()
			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := addTree(watcher, event.Name); err != nil {
						logf("watch new vault directory %q: %v", event.Name, err)
						needsReconciliation = true
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("vault watcher errors closed")
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				logf("vault watcher event queue overflow; reconciling: %v", err)
			} else {
				logf("vault watcher error; reconciling: %v", err)
			}
			needsReconciliation = true
		case <-ticker.C:
			logf("preview reconciliation scheduled")
			needsReconciliation = true
		case <-debounce:
			timer = nil
			debounce = nil
			if !building && len(pending) > 0 {
				startBuild("filesystem events")
			}
		case result := <-completed:
			building = false
			if result.err != nil {
				logf("preview build failed: %v", result.err)
			} else {
				logf("preview build finished")
			}
			// A failed snapshot is retried on the next event/reconciliation. Events
			// received while this build ran already have their own coalesced follow-up.
		}
	}
}

func validate(config Config) error {
	if config.Build == nil {
		return fmt.Errorf("preview build function is required")
	}
	if config.DebounceInterval <= 0 || config.ReconciliationInterval <= 0 {
		return fmt.Errorf("watch intervals must be positive")
	}
	info, err := os.Stat(config.VaultDirectory)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("vault directory %q is not accessible", config.VaultDirectory)
	}
	return nil
}

func addTree(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
}

func sortedPaths(paths map[string]struct{}) []string {
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
