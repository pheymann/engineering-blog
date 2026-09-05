package watchservice

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunReconcilesStartupAndCoalescesVaultChanges(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, "post.md"), "initial")
	builds := make(chan struct{}, 20)
	ctx, cancel := context.WithCancel(context.Background())
	done := runService(t, ctx, Config{
		VaultDirectory: vault, DebounceInterval: 100 * time.Millisecond, ReconciliationInterval: 5 * time.Second,
		Build: func() error { builds <- struct{}{}; return nil }, Logf: func(string, ...any) {},
	})
	defer stopService(t, cancel, done)
	waitBuild(t, builds) // startup reconciliation

	post := filepath.Join(vault, "post.md")
	for _, content := range []string{"first", "second", "final"} {
		writeFile(t, post, content)
	}
	waitBuild(t, builds)
	noBuild(t, builds, 250*time.Millisecond)

	temporary := filepath.Join(vault, ".post.md.tmp")
	writeFile(t, temporary, "rename final")
	if err := os.Rename(temporary, post); err != nil {
		t.Fatal(err)
	}
	waitBuild(t, builds)
	noBuild(t, builds, 250*time.Millisecond)

	if err := os.Remove(post); err != nil {
		t.Fatal(err)
	}
	waitBuild(t, builds)

	nested := filepath.Join(vault, "new", "nested")
	writeFile(t, filepath.Join(nested, "post.md"), "new directory content")
	waitBuild(t, builds)
	noBuild(t, builds, 250*time.Millisecond)
}

func TestRunPeriodicallyReconcilesMissedChanges(t *testing.T) {
	vault := t.TempDir()
	builds := make(chan struct{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := runService(t, ctx, Config{
		VaultDirectory: vault, DebounceInterval: time.Second, ReconciliationInterval: 70 * time.Millisecond,
		Build: func() error { builds <- struct{}{}; return nil }, Logf: func(string, ...any) {},
	})
	defer stopService(t, cancel, done)
	waitBuild(t, builds) // startup
	waitBuild(t, builds) // periodic full reconciliation, independent of events
}

func TestRunSerializesBuildsAndCoalescesEventsWhileBusy(t *testing.T) {
	vault := t.TempDir()
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	var mutex sync.Mutex
	active, maximum, calls := 0, 0, 0
	build := func() error {
		mutex.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		calls++
		call := calls
		mutex.Unlock()
		started <- struct{}{}
		if call == 1 {
			<-release
		}
		mutex.Lock()
		active--
		mutex.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runService(t, ctx, Config{
		VaultDirectory: vault, DebounceInterval: 70 * time.Millisecond, ReconciliationInterval: 5 * time.Second,
		Build: build, Logf: func(string, ...any) {},
	})
	defer stopService(t, cancel, done)
	waitBuild(t, started) // blocked startup build
	writeFile(t, filepath.Join(vault, "one.md"), "one")
	writeFile(t, filepath.Join(vault, "two.md"), "two")
	noBuild(t, started, 180*time.Millisecond)
	close(release)
	waitBuild(t, started) // one follow-up for both paths
	noBuild(t, started, 180*time.Millisecond)
	mutex.Lock()
	defer mutex.Unlock()
	if maximum != 1 || calls != 2 {
		t.Fatalf("build concurrency/calls = %d/%d, want 1/2", maximum, calls)
	}
}

func runService(t *testing.T, ctx context.Context, config Config) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, config) }()
	return done
}

func stopService(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch service did not stop")
	}
}

func waitBuild(t *testing.T, builds <-chan struct{}) {
	t.Helper()
	select {
	case <-builds:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for build")
	}
}

func noBuild(t *testing.T, builds <-chan struct{}, duration time.Duration) {
	t.Helper()
	select {
	case <-builds:
		t.Fatal("unexpected additional build")
	case <-time.After(duration):
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
