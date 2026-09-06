package gitpublisher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishCommitsOnlyDocsAndPushes(t *testing.T) {
	repository, remote := newRepository(t)
	write(t, filepath.Join(repository, "docs", "index.html"), "published")
	write(t, filepath.Join(repository, "README.md"), "staged source change")
	git(t, repository, "add", "README.md")
	write(t, filepath.Join(repository, "notes.txt"), "untracked source change")

	published, err := Publish(Config{RepositoryDirectory: repository, OutputDirectory: filepath.Join(repository, "docs")})
	if err != nil || !published {
		t.Fatalf("Publish() = %t, %v", published, err)
	}
	if got := git(t, repository, "show", "--format=", "--name-only", "HEAD"); got != "docs/index.html" {
		t.Fatalf("committed files = %q", got)
	}
	if got := git(t, repository, "status", "--short", "README.md", "notes.txt"); got != "M  README.md\n?? notes.txt" {
		t.Fatalf("unrelated statuses = %q", got)
	}
	if got := git(t, remote, "rev-parse", "main"); got == "" {
		t.Fatal("remote main was not pushed")
	}
}

func TestPublishRejectsPreexistingStagedDocsChanges(t *testing.T) {
	repository, _ := newRepository(t)
	write(t, filepath.Join(repository, "docs", "manual.html"), "operator-staged content")
	git(t, repository, "add", "docs/manual.html")
	before := git(t, repository, "rev-parse", "HEAD")

	published, err := Publish(Config{RepositoryDirectory: repository, OutputDirectory: filepath.Join(repository, "docs")})
	if err == nil || published {
		t.Fatalf("Publish() = %t, %v, want dirty-index conflict", published, err)
	}
	if after := git(t, repository, "rev-parse", "HEAD"); after != before {
		t.Fatalf("Publish committed pre-existing staged docs change: %s != %s", after, before)
	}
	if got := git(t, repository, "status", "--short", "docs/manual.html"); got != "A  docs/manual.html" {
		t.Fatalf("Publish changed staged docs status: %q", got)
	}
}

func TestPublishNoopDoesNotCommitOrPush(t *testing.T) {
	repository, _ := newRepository(t)
	if err := os.Mkdir(filepath.Join(repository, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := git(t, repository, "rev-parse", "HEAD")
	published, err := Publish(Config{RepositoryDirectory: repository, OutputDirectory: filepath.Join(repository, "docs")})
	if err != nil || published {
		t.Fatalf("Publish() = %t, %v", published, err)
	}
	if after := git(t, repository, "rev-parse", "HEAD"); after != before {
		t.Fatalf("commit changed from %s to %s", before, after)
	}
}

func TestPublishReportsPushFailureAndRetainsCommit(t *testing.T) {
	repository, _ := newRepository(t)
	git(t, repository, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	write(t, filepath.Join(repository, "docs", "index.html"), "changed")
	published, err := Publish(Config{RepositoryDirectory: repository, OutputDirectory: filepath.Join(repository, "docs")})
	if !published || err == nil || !strings.Contains(err.Error(), "push generated output") {
		t.Fatalf("Publish() = %t, %v", published, err)
	}
	if got := git(t, repository, "show", "--format=", "--name-only", "HEAD"); got != "docs/index.html" {
		t.Fatalf("recoverable commit files = %q", got)
	}
}

func newRepository(t *testing.T) (string, string) {
	t.Helper()
	repository := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, repository, "init", "-b", "main")
	git(t, repository, "config", "user.name", "Test Publisher")
	git(t, repository, "config", "user.email", "publisher@example.test")
	write(t, filepath.Join(repository, "README.md"), "base")
	git(t, repository, "add", "README.md")
	git(t, repository, "commit", "-m", "base")
	git(t, repository, "init", "--bare", remote)
	git(t, repository, "remote", "add", "origin", remote)
	return repository, remote
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func git(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
