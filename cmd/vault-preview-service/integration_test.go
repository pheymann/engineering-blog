//go:build integration

package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServiceStartsWithValidConfiguration(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	binary := filepath.Join(t.TempDir(), "vault-preview-service")
	build := exec.Command("go", "build", "-o", binary, "./cmd/vault-preview-service")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build service: %v\n%s", err, output)
	}

	vault := t.TempDir()
	output := filepath.Join(t.TempDir(), "site")
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "BLOG_VAULT_DIRECTORY="+vault, "BLOG_PREVIEW_OUTPUT_DIRECTORY="+output)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	started := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		started <- line
	}()
	select {
	case line := <-started:
		if !strings.Contains(line, "vault-preview-service started") {
			t.Fatalf("startup output = %q", line)
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("service did not start")
	}

	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("service exit: %v", err)
	}
}
