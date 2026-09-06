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
	repository := isolatedRepository(t, repositoryRoot)
	command := exec.Command(binary)
	command.Dir = repository
	command.Env = append(os.Environ(), "BLOG_VAULT_DIRECTORY="+vault, "BLOG_PREVIEW_OUTPUT_DIRECTORY="+output, "BLOG_PRODUCTION_OUTPUT_DIRECTORY="+filepath.Join(repository, "docs"), "BLOG_GIT_REPOSITORY_DIRECTORY="+repository)
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

func TestServiceBuildOncePublishesToIsolatedRemote(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	binary := filepath.Join(t.TempDir(), "vault-preview-service")
	build := exec.Command("go", "build", "-o", binary, "./cmd/vault-preview-service")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build service: %v\n%s", err, output)
	}
	repository := isolatedRepository(t, repositoryRoot)
	command := exec.Command(binary, "--build-once")
	command.Dir = repository
	command.Env = append(os.Environ(), "BLOG_VAULT_DIRECTORY="+t.TempDir(), "BLOG_PREVIEW_OUTPUT_DIRECTORY="+filepath.Join(t.TempDir(), "site"), "BLOG_PRODUCTION_OUTPUT_DIRECTORY="+filepath.Join(repository, "docs"), "BLOG_GIT_REPOSITORY_DIRECTORY="+repository)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build once: %v\n%s", err, output)
	}
	verify := exec.Command("git", "--git-dir", filepath.Join(repository, "remote.git"), "rev-parse", "main")
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("isolated remote was not pushed: %v\n%s", err, output)
	}
}

func TestServiceBuildOnceRetriesFailedPushWithoutAnotherBuildChange(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	binary := filepath.Join(t.TempDir(), "vault-preview-service")
	build := exec.Command("go", "build", "-o", binary, "./cmd/vault-preview-service")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build service: %v\n%s", err, output)
	}
	repository := isolatedRepository(t, repositoryRoot)
	remote, err := filepath.EvalSymlinks(filepath.Join(repository, "remote.git"))
	if err != nil {
		t.Fatal(err)
	}
	missingRemote := filepath.Join(t.TempDir(), "missing.git")
	setRemote(t, repository, missingRemote)
	vault, preview := t.TempDir(), filepath.Join(t.TempDir(), "site")
	run := func() ([]byte, error) {
		command := exec.Command(binary, "--build-once")
		command.Dir = repository
		command.Env = append(os.Environ(), "BLOG_VAULT_DIRECTORY="+vault, "BLOG_PREVIEW_OUTPUT_DIRECTORY="+preview, "BLOG_PRODUCTION_OUTPUT_DIRECTORY="+filepath.Join(repository, "docs"), "BLOG_GIT_REPOSITORY_DIRECTORY="+repository)
		return command.CombinedOutput()
	}
	if output, err := run(); err == nil || !strings.Contains(string(output), "push generated output") {
		t.Fatalf("first build = %v\n%s", err, output)
	}
	firstCommit := gitHead(t, repository)
	setRemote(t, repository, remote)
	if output, err := run(); err != nil {
		t.Fatalf("retry build: %v\n%s", err, output)
	}
	if after := gitHead(t, repository); after != firstCommit {
		t.Fatalf("retry created a second commit: %s != %s", after, firstCommit)
	}
	verify := exec.Command("git", "--git-dir", remote, "rev-parse", "main")
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("recovered push missing: %v\n%s", err, output)
	}
}

// TestServicePublishWorkflowUsesOnlyTemporaryVaultAndRemote covers the production
// tag transition end to end. Every path comes from t.TempDir, so it cannot touch
// the operator's vault, checkout, or origin remote.
func TestServicePublishWorkflowUsesOnlyTemporaryVaultAndRemote(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	binary := buildService(t, repositoryRoot)
	repository := isolatedRepository(t, repositoryRoot)
	vault := t.TempDir()
	preview := filepath.Join(t.TempDir(), "site")
	post := filepath.Join(vault, "Isolated Publish.md")

	run := func() {
		command := exec.Command(binary, "--build-once")
		command.Dir = repository
		command.Env = append(os.Environ(), "BLOG_VAULT_DIRECTORY="+vault, "BLOG_PREVIEW_OUTPUT_DIRECTORY="+preview, "BLOG_PRODUCTION_OUTPUT_DIRECTORY="+filepath.Join(repository, "docs"), "BLOG_GIT_REPOSITORY_DIRECTORY="+repository)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("build once: %v\n%s", err, output)
		}
	}
	writePost(t, post, "#blog #engineering #publish\nFirst published version.")
	run()
	firstCommit := gitHead(t, repository)
	assertRemotePage(t, repository, "First published version.")
	for _, file := range strings.Split(gitOutput(t, repository, "show", "--format=", "--name-only", "HEAD"), "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if !strings.HasPrefix(file, "docs/") {
			t.Fatalf("publication included non-docs file %q", file)
		}
	}

	// A reconciliation of unchanged input neither commits nor pushes.
	run()
	if after := gitHead(t, repository); after != firstCommit {
		t.Fatalf("no-op reconciliation committed %s after %s", after, firstCommit)
	}

	writePost(t, post, "#blog #engineering #publish\nUpdated published version.")
	run()
	updatedCommit := gitHead(t, repository)
	if updatedCommit == firstCommit {
		t.Fatal("updated publish did not create a commit")
	}
	assertRemotePage(t, repository, "Updated published version.")

	// Removing publish freezes the last public page. Later edits remain private.
	writePost(t, post, "#blog #engineering\nUnpublished change.")
	run()
	if after := gitHead(t, repository); after != updatedCommit {
		t.Fatalf("removing publish committed %s after %s", after, updatedCommit)
	}
	assertRemotePage(t, repository, "Updated published version.")
	writePost(t, post, "#blog #engineering\nAnother unpublished change.")
	run()
	if after := gitHead(t, repository); after != updatedCommit {
		t.Fatalf("unpublished edit committed %s after %s", after, updatedCommit)
	}
	assertRemotePage(t, repository, "Updated published version.")
}

func buildService(t *testing.T, repositoryRoot string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "vault-preview-service")
	build := exec.Command("go", "build", "-o", binary, "./cmd/vault-preview-service")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build service: %v\n%s", err, output)
	}
	return binary
}

func writePost(t *testing.T, path, body string) {
	t.Helper()
	content := "---\ndate: 2026-09-06\n---\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRemotePage(t *testing.T, repository, expected string) {
	t.Helper()
	remote, err := filepath.EvalSymlinks(filepath.Join(repository, "remote.git"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "--git-dir", remote, "show", "main:docs/isolated-publish/index.html")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("read published page: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), expected) {
		t.Fatalf("published page does not contain %q:\n%s", expected, output)
	}
}

func isolatedRepository(t *testing.T, source string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("create remote: %v\n%s", err, output)
	}
	repository := filepath.Join(t.TempDir(), "repository")
	clone := exec.Command("git", "clone", "--no-local", source, repository)
	if output, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone repository: %v\n%s", err, output)
	}
	for _, arguments := range [][]string{{"remote", "set-url", "origin", remote}, {"config", "user.name", "Integration Test"}, {"config", "user.email", "integration@example.test"}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("configure clone: %v\n%s", err, output)
		}
	}
	// Keep the bare remote path available to the caller without relying on its
	// original temporary parent.
	if err := os.Symlink(remote, filepath.Join(repository, "remote.git")); err != nil {
		t.Fatal(err)
	}
	return repository
}

func setRemote(t *testing.T, repository, remote string) {
	t.Helper()
	command := exec.Command("git", "remote", "set-url", "origin", remote)
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("set remote: %v\n%s", err, output)
	}
}

func gitHead(t *testing.T, repository string) string {
	return gitOutput(t, repository, "rev-parse", "HEAD")
}

func gitOutput(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repository
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
