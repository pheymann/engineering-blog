package previewbuilder

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const mermaidRenderTimeout = 30 * time.Second

type fence struct {
	char   byte
	length int
	info   string
}

func parseOpeningFence(line string) (fence, bool) {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line) || (line[i] != '`' && line[i] != '~') {
		return fence{}, false
	}
	c, end := line[i], i
	for end < len(line) && line[end] == c {
		end++
	}
	if end-i < 3 {
		return fence{}, false
	}
	info := strings.TrimSpace(line[end:])
	if c == '`' && strings.Contains(info, "`") {
		return fence{}, false
	}
	return fence{c, end - i, info}, true
}
func isClosingFence(line string, open fence) bool {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line) || line[i] != open.char {
		return false
	}
	end := i
	for end < len(line) && line[end] == open.char {
		end++
	}
	return end-i >= open.length && strings.TrimSpace(line[end:]) == ""
}
func isMermaidInfo(info string) bool {
	fields := strings.Fields(info)
	return len(fields) > 0 && strings.EqualFold(fields[0], "mermaid")
}

func renderMermaidDiagrams(body, postPath, slug, outputDirectory, command, puppeteerConfig, renderUser string) (string, map[string]bool, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	rendered := make([]string, 0, len(lines))
	used := map[string]bool{}
	for index := 0; index < len(lines); {
		open, ok := parseOpeningFence(lines[index])
		if !ok {
			rendered = append(rendered, lines[index])
			index++
			continue
		}
		start := index
		index++
		var definition []string
		for index < len(lines) && !isClosingFence(lines[index], open) {
			definition = append(definition, lines[index])
			index++
		}
		if index == len(lines) {
			if isMermaidInfo(open.info) {
				return "", nil, fmt.Errorf("%s: Mermaid block at line %d is not terminated", postPath, start+1)
			}
			rendered = append(rendered, lines[start:]...)
			break
		}
		index++
		if !isMermaidInfo(open.info) {
			rendered = append(rendered, lines[start:index]...)
			continue
		}
		source := strings.TrimSpace(strings.Join(definition, "\n"))
		if source == "" {
			return "", nil, fmt.Errorf("%s: Mermaid block at line %d is empty", postPath, start+1)
		}
		hash := sha256.Sum256([]byte(source))
		filename := fmt.Sprintf("%s-%x.svg", slug, hash[:8])
		target := filepath.Join(outputDirectory, filename)
		if !validSVG(target) {
			if err := renderMermaidSVG(source, target, command, puppeteerConfig, renderUser); err != nil {
				return "", nil, fmt.Errorf("%s: render Mermaid block at line %d: %w", postPath, start+1, err)
			}
		}
		used[filename] = true
		rendered = append(rendered, "![Mermaid diagram](/assets/diagrams/"+filename+")")
	}
	return strings.Join(rendered, "\n"), used, nil
}
func validSVG(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "<svg")
}

func renderMermaidSVG(source, outputPath, command, puppeteerConfig, renderUser string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(outputPath), ".mermaid-render-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	credential, err := rendererCredential(renderUser)
	if err != nil {
		return err
	}
	if credential != nil {
		if err := os.Chown(work, int(credential.Uid), int(credential.Gid)); err != nil {
			return err
		}
		if err := os.Chmod(work, 0o700); err != nil {
			return err
		}
	}
	inputPath, temporaryOutput := filepath.Join(work, "diagram.mmd"), filepath.Join(work, "diagram.svg")
	if err := os.WriteFile(inputPath, []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	if credential != nil {
		if err := os.Chown(inputPath, int(credential.Uid), int(credential.Gid)); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), mermaidRenderTimeout)
	defer cancel()
	process := exec.CommandContext(ctx, command, "-q", "-i", inputPath, "-o", temporaryOutput, "-b", "transparent", "-p", puppeteerConfig)
	if credential != nil {
		process.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		process.Dir = work
		process.Env = append(os.Environ(), "HOME="+work, "XDG_CONFIG_HOME="+work, "XDG_CACHE_HOME="+work)
	}
	output, err := process.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("renderer timed out after %s", mermaidRenderTimeout)
	}
	if err != nil {
		return fmt.Errorf("renderer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if !validSVG(temporaryOutput) {
		return fmt.Errorf("renderer did not produce an SVG")
	}
	if err := os.Chmod(temporaryOutput, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporaryOutput, outputPath); err != nil {
		return fmt.Errorf("install generated SVG: %w", err)
	}
	return nil
}
func rendererCredential(name string) (*syscall.Credential, error) {
	if strings.TrimSpace(name) == "" {
		return nil, nil
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("Mermaid render user %q requires root transformer", name)
	}
	account, err := user.Lookup(name)
	if err != nil {
		return nil, fmt.Errorf("look up Mermaid render user %q: %w", name, err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	if uid == 0 {
		return nil, fmt.Errorf("Mermaid render user must be unprivileged")
	}
	groupIDs, err := account.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("look up Mermaid render groups: %w", err)
	}
	groups := make([]uint32, 0, len(groupIDs))
	for _, value := range groupIDs {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, err
		}
		groups = append(groups, uint32(parsed))
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups}, nil
}
