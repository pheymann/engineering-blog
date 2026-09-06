// Package gitpublisher commits and pushes generated production output.
package gitpublisher

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const commitMessage = "Publish generated site"

// Config identifies the checkout and generated directory to publish.
type Config struct {
	RepositoryDirectory string
	OutputDirectory     string
}

// Publish stages only OutputDirectory, commits it when necessary, and pushes
// that commit to origin/main. It never modifies source or unrelated files.
func Publish(config Config) (bool, error) {
	repository, output, err := normalize(config)
	if err != nil {
		return false, err
	}
	if err := run(repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return false, fmt.Errorf("verify Git repository: %w", err)
	}
	staged, err := hasStagedOutput(repository, output)
	if err != nil {
		return false, err
	}
	if staged {
		return false, fmt.Errorf("refuse to publish: generated output already has staged changes; resolve the dirty Git index first")
	}
	if err := run(repository, "add", "--", output); err != nil {
		return false, fmt.Errorf("stage generated output: %w", err)
	}
	changed, err := hasStagedOutput(repository, output)
	if err != nil {
		return false, err
	}
	if changed {
		if err := run(repository, "var", "GIT_AUTHOR_IDENT"); err != nil {
			return false, fmt.Errorf("Git author identity is not configured: %w", err)
		}
		// --only prevents pre-existing staged changes outside docs from becoming
		// part of the generated-site commit.
		if err := run(repository, "commit", "--only", "-m", commitMessage, "--", output); err != nil {
			return false, fmt.Errorf("commit generated output: %w", err)
		}
	} else if !pendingPublish(repository, output) {
		return false, nil
	}
	if err := run(repository, "push", "origin", "HEAD:main"); err != nil {
		return changed, fmt.Errorf("push generated output to origin/main: %w", err)
	}
	return true, nil
}

// pendingPublish recognizes only the docs-only commit this package created.
// A fetch lets a later reconciliation distinguish a failed push from a commit
// that another publisher has already incorporated into origin/main.
func pendingPublish(repository, output string) bool {
	if outputOf(repository, "log", "-1", "--format=%s") != commitMessage || !headContainsOnly(repository, output) {
		return false
	}
	if err := run(repository, "fetch", "origin", "main"); err != nil {
		return true
	}
	command := exec.Command("git", "merge-base", "--is-ancestor", "HEAD", "FETCH_HEAD")
	command.Dir = repository
	return command.Run() != nil
}

func headContainsOnly(repository, output string) bool {
	files := strings.Fields(outputOf(repository, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"))
	if len(files) == 0 {
		return false
	}
	for _, file := range files {
		if file != output && !strings.HasPrefix(file, output+"/") {
			return false
		}
	}
	return true
}

func outputOf(directory string, arguments ...string) string {
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func normalize(config Config) (string, string, error) {
	if strings.TrimSpace(config.RepositoryDirectory) == "" || strings.TrimSpace(config.OutputDirectory) == "" {
		return "", "", fmt.Errorf("repository directory and output directory are required")
	}
	repository, err := filepath.Abs(config.RepositoryDirectory)
	if err != nil {
		return "", "", err
	}
	output, err := filepath.Rel(repository, config.OutputDirectory)
	if err != nil {
		return "", "", fmt.Errorf("resolve output directory: %w", err)
	}
	if output == "." || output == ".." || strings.HasPrefix(output, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("output directory %q must be inside repository %q", config.OutputDirectory, repository)
	}
	return repository, filepath.ToSlash(output), nil
}

func hasStagedOutput(repository, output string) (bool, error) {
	command := exec.Command("git", "diff", "--cached", "--quiet", "--", output)
	command.Dir = repository
	if err := command.Run(); err == nil {
		return false, nil
	} else if status, ok := err.(*exec.ExitError); ok && status.ExitCode() == 1 {
		return true, nil
	} else {
		return false, fmt.Errorf("inspect staged generated output: %w", err)
	}
}

func run(directory string, arguments ...string) error {
	command := exec.Command("git", arguments...)
	command.Dir = directory
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}
