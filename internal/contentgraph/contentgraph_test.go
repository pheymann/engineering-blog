package contentgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pheymann/engineering-blog/internal/post"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(t *testing.T, root string) []*post.Post
		wantLinks []string
		wantAsset string
		wantError string
	}{
		{
			name: "resolves post links and image assets",
			prepare: func(t *testing.T, root string) []*post.Post {
				writeFile(t, filepath.Join(root, "assets", "diagram.png"))
				return []*post.Post{
					fixturePost("first.md", "First", "first", "See [[Second Post|the second post]].\n![diagram](assets/diagram.png)"),
					fixturePost("second.md", "Second Post", "second-post", "content"),
				}
			},
			wantLinks: []string{"/second-post/"},
			wantAsset: "diagram.png",
		},
		{
			name: "rejects broken post link",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "first", "[[Missing]]")}
			},
			wantError: "first.md: broken post link \"Missing\"",
		},
		{
			name: "rejects missing image",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "first", "![missing](assets/missing.png)")}
			},
			wantError: "first.md: image \"assets/missing.png\" is missing",
		},
		{
			name: "resolves a unique image filename anywhere in the vault",
			prepare: func(t *testing.T, root string) []*post.Post {
				writeFile(t, filepath.Join(root, "attachments", "diagram.png"))
				return []*post.Post{fixturePost("first.md", "First", "first", "![diagram](diagram.png)")}
			},
			wantAsset: "diagram.png",
		},
		{
			name: "rejects ambiguous image filename",
			prepare: func(t *testing.T, root string) []*post.Post {
				writeFile(t, filepath.Join(root, "one", "diagram.png"))
				writeFile(t, filepath.Join(root, "two", "diagram.png"))
				return []*post.Post{fixturePost("first.md", "First", "first", "![diagram](diagram.png)")}
			},
			wantError: "first.md: image \"diagram.png\" is ambiguous between",
		},
		{
			name: "rejects duplicate slugs",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "same", ""), fixturePost("second.md", "Second", "same", "")}
			},
			wantError: "second.md: slug \"same\" conflicts with first.md",
		},
		{
			name: "rejects duplicate asset filename",
			prepare: func(t *testing.T, root string) []*post.Post {
				writeFile(t, filepath.Join(root, "one", "chart.png"))
				writeFile(t, filepath.Join(root, "two", "chart.png"))
				return []*post.Post{
					fixturePost("first.md", "First", "first", "![one](one/chart.png)"),
					fixturePost("second.md", "Second", "second", "![two](two/chart.png)"),
				}
			},
			wantError: "second.md: asset filename \"chart.png\" conflicts with",
		},
		{
			name: "rejects redirect owned by another post",
			prepare: func(t *testing.T, root string) []*post.Post {
				first := fixturePost("first.md", "First", "first", "")
				first.Redirects = []string{"/old/"}
				second := fixturePost("second.md", "Second", "second", "")
				second.Redirects = []string{"/old/"}
				return []*post.Post{first, second}
			},
			wantError: "second.md: redirect \"/old/\" conflicts with first.md",
		},
		{
			name: "rejects redirect that conflicts with current URL",
			prepare: func(t *testing.T, root string) []*post.Post {
				first := fixturePost("first.md", "First", "first", "")
				second := fixturePost("second.md", "Second", "second", "")
				second.Redirects = []string{"/first/"}
				return []*post.Post{first, second}
			},
			wantError: "second.md: redirect \"/first/\" conflicts with first.md",
		},
		{
			name: "rejects equivalent redirect without trailing slash",
			prepare: func(t *testing.T, root string) []*post.Post {
				first := fixturePost("first.md", "First", "first", "")
				second := fixturePost("second.md", "Second", "second", "")
				second.Redirects = []string{"/first"}
				return []*post.Post{first, second}
			},
			wantError: "second.md: redirect \"/first\" conflicts with first.md",
		},
		{
			name: "rejects Obsidian embed",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "first", "![[image.png]]")}
			},
			wantError: "first.md: Obsidian embeds are not supported",
		},
		{
			name: "rejects Obsidian callout",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "first", "> [!NOTE]\n> text")}
			},
			wantError: "first.md: Obsidian callouts are not supported",
		},
		{
			name: "rejects Excalidraw content",
			prepare: func(t *testing.T, root string) []*post.Post {
				return []*post.Post{fixturePost("first.md", "First", "first", "excalidraw-plugin: parsed")}
			},
			wantError: "first.md: Excalidraw content is not supported",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			graph, err := Validate(root, test.prepare(t, root))
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Validate() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if len(graph.Links["first.md"]) != len(test.wantLinks) {
				t.Fatalf("links = %#v, want %d", graph.Links, len(test.wantLinks))
			}
			for index, want := range test.wantLinks {
				if graph.Links["first.md"][index].URL != want {
					t.Fatalf("link %d URL = %q, want %q", index, graph.Links["first.md"][index].URL, want)
				}
			}
			if test.wantAsset != "" && (len(graph.Assets) != 1 || graph.Assets[0].Filename != test.wantAsset) {
				t.Fatalf("assets = %#v, want %q", graph.Assets, test.wantAsset)
			}
		})
	}
}

func fixturePost(path, title, slug, body string) *post.Post {
	return &post.Post{Path: path, Title: title, Slug: slug, Body: body}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
}
