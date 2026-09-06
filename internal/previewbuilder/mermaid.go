package previewbuilder

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const mermaidRenderTimeout = 30 * time.Second

func renderMermaidDiagrams(body, postPath, slug, outputDirectory, command, puppeteerConfig string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var rendered []string
	for index := 0; index < len(lines); {
		if strings.TrimSpace(lines[index]) != "```mermaid" {
			rendered = append(rendered, lines[index])
			index++
			continue
		}

		start := index
		index++
		var definition []string
		for index < len(lines) && strings.TrimSpace(lines[index]) != "```" {
			definition = append(definition, lines[index])
			index++
		}
		if index == len(lines) {
			return "", fmt.Errorf("%s: Mermaid block at line %d is not terminated", postPath, start+1)
		}
		index++
		source := strings.TrimSpace(strings.Join(definition, "\n"))
		if source == "" {
			return "", fmt.Errorf("%s: Mermaid block at line %d is empty", postPath, start+1)
		}
		hash := sha256.Sum256([]byte(source))
		filename := fmt.Sprintf("%s-%x.svg", slug, hash[:8])
		if err := renderMermaidSVG(source, filepath.Join(outputDirectory, filename), command, puppeteerConfig); err != nil {
			return "", fmt.Errorf("%s: render Mermaid block at line %d: %w", postPath, start+1, err)
		}
		rendered = append(rendered, "![Mermaid diagram](/assets/diagrams/"+filename+")")
	}
	return strings.Join(rendered, "\n"), nil
}

func renderMermaidSVG(source, outputPath, command, puppeteerConfig string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create diagram directory: %w", err)
	}
	input, err := os.CreateTemp(filepath.Dir(outputPath), ".mermaid-*.mmd")
	if err != nil {
		return fmt.Errorf("create temporary Mermaid input: %w", err)
	}
	inputPath := input.Name()
	defer os.Remove(inputPath)
	if _, err := input.WriteString(source + "\n"); err != nil {
		input.Close()
		return fmt.Errorf("write temporary Mermaid input: %w", err)
	}
	if err := input.Close(); err != nil {
		return fmt.Errorf("close temporary Mermaid input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), mermaidRenderTimeout)
	defer cancel()
	process := exec.CommandContext(ctx, command, "-q", "-i", inputPath, "-o", outputPath, "-b", "transparent", "-p", puppeteerConfig)
	output, err := process.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("renderer timed out after %s", mermaidRenderTimeout)
	}
	if err != nil {
		return fmt.Errorf("renderer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	generated, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("read generated SVG: %w", err)
	}
	if !strings.Contains(string(generated), "<svg") {
		return fmt.Errorf("renderer did not produce an SVG")
	}
	return nil
}
