package previewbuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMermaidFenceRecognition(t *testing.T) {
	command := filepath.Join(t.TempDir(), "mmdc")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then printf '<svg></svg>' > \"$2\"; exit; fi; shift; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, body string
		count      int
	}{
		{"tilde", "~~~mermaid\ngraph TD\nA-->B\n~~~", 1},
		{"long", "`````mermaid\ngraph TD\nA-->B\n`````", 1},
		{"nested example", "````markdown\n```mermaid\ngraph TD\n```\n````", 0},
		{"indented sample", "    ```mermaid\n    graph TD\n    ```", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, used, err := renderMermaidDiagrams(tc.body, "post.md", "post", filepath.Join(t.TempDir(), "diagrams"), command, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if len(used) != tc.count {
				t.Fatalf("used=%d body=%q", len(used), body)
			}
		})
	}
}

func TestMermaidCacheDoesNotInvokeRenderer(t *testing.T) {
	dir := t.TempDir()
	source := "graph TD\nA-->B"
	hashName := diagramName("post", source)
	if err := os.WriteFile(filepath.Join(dir, hashName), []byte("<svg>cached</svg>"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _, err := renderMermaidDiagrams("```mermaid\n"+source+"\n```", "post.md", "post", dir, "/does/not/exist", "", "")
	if err != nil || !strings.Contains(body, hashName) {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestPruneDiagramsKeepsOnlyReferencedSVGs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"current.svg", "preserved.svg", "stale.svg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("<svg></svg>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneDiagrams(dir, map[string]bool{"current.svg": true, "preserved.svg": true}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"current.svg", "preserved.svg"} {
		if !validSVG(filepath.Join(dir, name)) {
			t.Fatalf("%s was pruned", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.svg")); !os.IsNotExist(err) {
		t.Fatalf("stale.svg still exists: %v", err)
	}
}

func diagramName(slug, source string) string {
	// Ask the production naming path rather than duplicating its hash details.
	dir, _ := os.MkdirTemp("", "diagram-name-")
	defer os.RemoveAll(dir)
	command := filepath.Join(dir, "mmdc")
	_ = os.WriteFile(command, []byte("#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then printf '<svg></svg>' > \"$2\"; exit; fi; shift; done\n"), 0o755)
	_, used, _ := renderMermaidDiagrams("```mermaid\n"+source+"\n```", "post.md", slug, dir, command, "", "")
	for name := range used {
		return name
	}
	return ""
}
