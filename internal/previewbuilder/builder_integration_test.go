package previewbuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildLifecycle(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "preview")
	writeSharedSite(t, static)
	write(t, filepath.Join(vault, "images", "diagram.png"), "image")
	write(t, filepath.Join(vault, "First post.md"), postSource("First post", "2026-09-04", "/first-old/", "#blog #engineering #preview\n\nSee [[Second post]].\n\n![[diagram.png]]\n\nfirst body"))
	write(t, filepath.Join(vault, "Second post.md"), postSource("Second post", "2026-09-03", "", "#blog #engineering #preview\nsecond body"))
	config := Config{VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output}

	result, err := Build(config)
	if err != nil {
		t.Fatalf("initial Build() error = %v", err)
	}
	if !result.Changed || result.Posts != 2 {
		t.Fatalf("initial Build() = %#v", result)
	}
	for _, path := range []string{"index.html", "first-post/index.html", "second-post/index.html", "first-old/index.html", "assets/posts/diagram.png", "assets/styles.css", "impressum/index.html", "datenschutzerklaerung/index.html", "robots.txt", "sitemap.xml"} {
		if _, err := os.Stat(filepath.Join(output, path)); err != nil {
			t.Errorf("initial output missing %s: %v", path, err)
		}
	}
	first := read(t, filepath.Join(output, "first-post", "index.html"))
	if !strings.Contains(first, `href="/second-post/"`) || !strings.Contains(first, `src="/assets/posts/diagram.png"`) {
		t.Fatalf("first post did not resolve graph references: %s", first)
	}
	if !strings.Contains(read(t, filepath.Join(output, "sitemap.xml")), "first-post") {
		t.Fatal("sitemap does not list generated post")
	}

	statePath := filepath.Join(root, stateFilename)
	beforeState := read(t, statePath)
	beforeInfo, err := os.Stat(filepath.Join(output, "second-post", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	result, err = Build(config)
	if err != nil {
		t.Fatalf("unchanged Build() error = %v", err)
	}
	if result.Changed {
		t.Fatalf("unchanged Build() = %#v, want no mutation", result)
	}
	afterInfo, err := os.Stat(filepath.Join(output, "second-post", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) || read(t, statePath) != beforeState {
		t.Fatal("unchanged build rewrote output or state")
	}

	write(t, filepath.Join(vault, "First post.md"), postSource("First post", "2026-09-04", "/first-old/", "#blog #engineering #preview\nupdated first body"))
	result, err = Build(config)
	if err != nil || !result.Changed {
		t.Fatalf("changed Build() = %#v, %v", result, err)
	}
	if !strings.Contains(read(t, filepath.Join(output, "first-post", "index.html")), "updated first body") {
		t.Fatal("changed post was not regenerated")
	}
	if read(t, filepath.Join(output, "second-post", "index.html")) == "" {
		t.Fatal("unaffected post was not retained")
	}

	write(t, filepath.Join(vault, "First post.md"), "# First post\n#blog #engineering\nno longer preview")
	result, err = Build(config)
	if err != nil || !result.Changed || result.Posts != 1 {
		t.Fatalf("retag Build() = %#v, %v", result, err)
	}
	for _, path := range []string{"first-post/index.html", "first-old/index.html"} {
		if _, err := os.Stat(filepath.Join(output, path)); !os.IsNotExist(err) {
			t.Errorf("retagged post output %s remains: %v", path, err)
		}
	}

	previousHome := read(t, filepath.Join(output, "index.html"))
	write(t, filepath.Join(vault, "Second post.md"), "# Second post\n#blog #engineering #preview")
	if _, err := Build(config); err == nil || !strings.Contains(err.Error(), "date property is required") {
		t.Fatalf("invalid Build() error = %v", err)
	}
	if got := read(t, filepath.Join(output, "index.html")); got != previousHome {
		t.Fatal("failed build replaced the last valid preview")
	}
}

func TestProductionBuildPublishesOnlyPublishTagsAndFreezesRemovedTag(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "docs")
	state := filepath.Join(root, "production-state.json")
	writeSharedSite(t, static)
	write(t, filepath.Join(static, "CNAME"), "engineering.paulheymann.de\n")
	published := filepath.Join(vault, "Published.md")
	write(t, published, postSource("Published", "2026-09-05", "", "#blog #engineering #publish\nPublished body"))
	write(t, filepath.Join(vault, "Preview only.md"), postSource("Preview only", "2026-09-06", "", "#blog #engineering #preview\nPrivate body"))
	config := Config{
		VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output,
		StatePath: state, DeploymentTag: "publish", PreserveRemoved: true,
	}

	result, err := Build(config)
	if err != nil || !result.Changed || result.Posts != 1 {
		t.Fatalf("initial production Build() = %#v, %v", result, err)
	}
	for _, path := range []string{"index.html", "published/index.html", "CNAME", "assets/styles.css"} {
		if _, err := os.Stat(filepath.Join(output, path)); err != nil {
			t.Errorf("production output missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "preview-only", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("preview-only post entered production: %v", err)
	}

	pageBefore := read(t, filepath.Join(output, "published", "index.html"))
	homeBefore := read(t, filepath.Join(output, "index.html"))
	write(t, published, postSource("Published", "2026-09-05", "", "#blog #engineering\nUnpublished edit"))
	result, err = Build(config)
	if err != nil {
		t.Fatalf("freeze Build() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("removing publish should record the new production selection")
	}
	if read(t, filepath.Join(output, "published", "index.html")) != pageBefore || read(t, filepath.Join(output, "index.html")) != homeBefore {
		t.Fatal("removing publish changed the last published output")
	}

	outputInfo, err := os.Stat(filepath.Join(output, "published", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	stateBefore := read(t, state)
	time.Sleep(10 * time.Millisecond)
	write(t, published, postSource("Published", "2026-09-05", "", "#blog #engineering\nAnother unpublished edit"))
	result, err = Build(config)
	if err != nil || result.Changed {
		t.Fatalf("unpublished edit Build() = %#v, %v, want no change", result, err)
	}
	outputAfter, err := os.Stat(filepath.Join(output, "published", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !outputAfter.ModTime().Equal(outputInfo.ModTime()) || read(t, state) != stateBefore {
		t.Fatal("unpublished edit rewrote production output or state")
	}

	write(t, filepath.Join(vault, "Broken.md"), "#blog #engineering #publish\nBroken")
	if _, err := Build(config); err == nil || !strings.Contains(err.Error(), "date property is required") {
		t.Fatalf("invalid production Build() error = %v", err)
	}
	if read(t, filepath.Join(output, "published", "index.html")) != pageBefore {
		t.Fatal("failed production build partially replaced output")
	}
}

func TestBuildDetectsRedirectOnlyChanges(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "preview")
	writeSharedSite(t, static)
	postPath := filepath.Join(vault, "Post.md")
	write(t, postPath, postSource("Post", "2026-09-05", "/old-one/", "#blog #engineering #preview\nBody"))
	config := Config{VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output}

	if _, err := Build(config); err != nil {
		t.Fatalf("initial Build() error = %v", err)
	}
	write(t, postPath, postSource("Post", "2026-09-05", "/old-two/", "#blog #engineering #preview\nBody"))
	result, err := Build(config)
	if err != nil {
		t.Fatalf("redirect-only Build() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("redirect-only Build() reported no change")
	}
	if _, err := os.Stat(filepath.Join(output, "old-two", "index.html")); err != nil {
		t.Fatalf("new redirect was not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "old-one", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("removed redirect remains in output: %v", err)
	}
}

func TestBuildPreservesUnaffectedGeneratedPost(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "preview")
	writeSharedSite(t, static)
	firstPath := filepath.Join(vault, "First.md")
	write(t, firstPath, postSource("First", "2026-09-05", "", "#blog #engineering #preview\nFirst body"))
	write(t, filepath.Join(vault, "Second.md"), postSource("Second", "2026-09-04", "", "#blog #engineering #preview\nSecond body"))
	config := Config{VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output}

	if _, err := Build(config); err != nil {
		t.Fatalf("initial Build() error = %v", err)
	}
	unaffectedPath := filepath.Join(output, "second", "index.html")
	before, err := os.Stat(unaffectedPath)
	if err != nil {
		t.Fatal(err)
	}
	write(t, firstPath, postSource("First", "2026-09-05", "", "#blog #engineering #preview\nUpdated first body"))
	if _, err := Build(config); err != nil {
		t.Fatalf("changed Build() error = %v", err)
	}
	after, err := os.Stat(unaffectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("editing one post replaced an unrelated generated post")
	}
}

func TestBuildRepairsMissingGeneratedFileWithUnchangedVault(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "preview")
	writeSharedSite(t, static)
	write(t, filepath.Join(vault, "Post.md"), postSource("Post", "2026-09-05", "", "#blog #engineering #preview\nBody"))
	config := Config{VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output}

	if _, err := Build(config); err != nil {
		t.Fatalf("initial Build() error = %v", err)
	}
	generated := filepath.Join(output, "post", "index.html")
	if err := os.Remove(generated); err != nil {
		t.Fatal(err)
	}
	result, err := Build(config)
	if err != nil {
		t.Fatalf("repair Build() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Build() did not detect missing generated post")
	}
	if _, err := os.Stat(generated); err != nil {
		t.Fatalf("missing generated post was not repaired: %v", err)
	}
	if err := os.WriteFile(generated, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = Build(config)
	if err != nil {
		t.Fatalf("corrupt-output repair Build() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Build() did not detect corrupt generated post")
	}
	if strings.Contains(read(t, generated), "corrupt") {
		t.Fatal("corrupt generated post was not repaired")
	}
}

func TestBuildRendersObsidianImageEmbedWithSpaces(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	static := filepath.Join(root, "static")
	output := filepath.Join(root, "preview")
	writeSharedSite(t, static)
	write(t, filepath.Join(vault, "My diagram.png"), "image")
	write(t, filepath.Join(vault, "Post.md"), postSource("Post", "2026-09-05", "", "#blog #engineering #preview\n\n![[My diagram.png]]"))

	if _, err := Build(Config{VaultDirectory: vault, StaticDirectory: static, OutputDirectory: output}); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	page := read(t, filepath.Join(output, "post", "index.html"))
	if !strings.Contains(page, `src="/assets/posts/My%20diagram.png"`) {
		t.Fatalf("generated post does not render the spaced image filename: %s", page)
	}
}

func writeSharedSite(t *testing.T, root string) {
	t.Helper()
	write(t, filepath.Join(root, "assets", "styles.css"), "body{}")
	write(t, filepath.Join(root, "assets", "images", "pauls-engineering-blog.svg"), "<svg></svg>")
	write(t, filepath.Join(root, "impressum", "index.html"), "impressum")
	write(t, filepath.Join(root, "datenschutzerklaerung", "index.html"), "privacy")
}

func postSource(title, date, redirect, body string) string {
	frontMatter := "---\ndate: " + date + "\n"
	if redirect != "" {
		frontMatter += "redirect:\n  - " + redirect + "\n"
	}
	return frontMatter + "---\n# " + title + "\n" + body
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
func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
