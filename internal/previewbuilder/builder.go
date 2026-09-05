// Package previewbuilder creates a complete local preview from a vault snapshot.
package previewbuilder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pheymann/engineering-blog/internal/contentgraph"
	"github.com/pheymann/engineering-blog/internal/post"
	"github.com/pheymann/engineering-blog/internal/render"
)

const stateFilename = ".engineering-blog-preview-state.json"

var (
	wikilink      = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	image         = regexp.MustCompile(`(!\[[^\]]*\]\()([^\s)]+)`)
	obsidianImage = regexp.MustCompile(`!\[\[([^\]|]+)(?:\|([^\]]*))?\]\]`)
)

// Config identifies the immutable vault input, shared site source, and preview output.
type Config struct {
	VaultDirectory  string
	StaticDirectory string
	OutputDirectory string
	StatePath       string
}

// Result describes whether Build committed a new preview.
type Result struct {
	Changed bool
	Posts   int
}

type buildState struct {
	Fingerprint string               `json:"fingerprint"`
	Posts       map[string]postState `json:"posts"`
	Files       map[string]string    `json:"files"`
}

type postState struct {
	Fingerprint string   `json:"fingerprint"`
	Slug        string   `json:"slug"`
	Redirects   []string `json:"redirects"`
}

// Build validates a complete vault snapshot before atomically replacing output.
// An identical snapshot leaves both the output and state file untouched.
func Build(config Config) (Result, error) {
	config, err := normalizedConfig(config)
	if err != nil {
		return Result{}, err
	}
	posts, err := loadPosts(config.VaultDirectory)
	if err != nil {
		return Result{}, err
	}
	graph, err := contentgraph.Validate(config.VaultDirectory, posts)
	if err != nil {
		return Result{}, err
	}
	fingerprint, err := fingerprintFor(posts, graph, config.StaticDirectory)
	if err != nil {
		return Result{}, err
	}
	previous, stateErr := readState(config.StatePath)
	sameInput := stateErr == nil && previous.Fingerprint == fingerprint
	outputIntact := stateErr == nil && outputMatches(config.OutputDirectory, previous.Files)
	if sameInput && outputIntact {
		return Result{Posts: len(posts)}, nil
	}

	stage, err := os.MkdirTemp(filepath.Dir(config.OutputDirectory), ".preview-stage-")
	if err != nil {
		return Result{}, fmt.Errorf("create preview stage: %w", err)
	}
	defer os.RemoveAll(stage)
	// The transformer runs as root for the private vault, while the web server
	// is unprivileged and must traverse the committed generated directory.
	if err := os.Chmod(stage, 0o755); err != nil {
		return Result{}, fmt.Errorf("prepare preview stage permissions: %w", err)
	}
	if stateErr == nil && directoryExists(config.OutputDirectory) {
		if err := hardlinkTree(config.OutputDirectory, stage); err != nil {
			return Result{}, fmt.Errorf("stage existing preview: %w", err)
		}
	}
	if err := populate(stage, config.StaticDirectory, posts, graph, previous, !outputIntact); err != nil {
		return Result{}, err
	}
	files, err := fileManifest(stage)
	if err != nil {
		return Result{}, fmt.Errorf("record staged preview: %w", err)
	}
	if err := replaceDirectory(stage, config.OutputDirectory); err != nil {
		return Result{}, err
	}
	if err := writeState(config.StatePath, buildState{Fingerprint: fingerprint, Posts: postStates(posts), Files: files}); err != nil {
		return Result{}, fmt.Errorf("write build state: %w", err)
	}
	return Result{Changed: true, Posts: len(posts)}, nil
}

func normalizedConfig(config Config) (Config, error) {
	for name, value := range map[string]string{"vault directory": config.VaultDirectory, "static directory": config.StaticDirectory, "output directory": config.OutputDirectory} {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}
	var err error
	config.VaultDirectory, err = filepath.Abs(config.VaultDirectory)
	if err != nil {
		return Config{}, err
	}
	config.StaticDirectory, err = filepath.Abs(config.StaticDirectory)
	if err != nil {
		return Config{}, err
	}
	config.OutputDirectory, err = filepath.Abs(config.OutputDirectory)
	if err != nil {
		return Config{}, err
	}
	if config.StatePath == "" {
		config.StatePath = filepath.Join(filepath.Dir(config.OutputDirectory), stateFilename)
	}
	statePath, err := filepath.Abs(config.StatePath)
	if err != nil {
		return Config{}, err
	}
	config.StatePath = statePath
	for _, directory := range []string{config.VaultDirectory, config.StaticDirectory} {
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			return Config{}, fmt.Errorf("%s is not a readable directory", directory)
		}
	}
	return config, nil
}

func loadPosts(root string) ([]*post.Post, error) {
	var posts []*post.Post
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parsed, err := post.Parse(filepath.ToSlash(relative), string(content))
		if err != nil {
			return err
		}
		if parsed != nil {
			posts = append(posts, parsed)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load vault posts: %w", err)
	}
	sort.Slice(posts, func(first, second int) bool { return posts[first].Path < posts[second].Path })
	return posts, nil
}

func populate(stage, static string, posts []*post.Post, graph *contentgraph.Graph, previous buildState, renderAll bool) error {
	current := postStates(posts)
	for _, old := range previous.Posts {
		if now, found := current[old.Slug]; !found || now.Fingerprint != old.Fingerprint {
			if err := os.RemoveAll(filepath.Join(stage, old.Slug)); err != nil {
				return err
			}
		}
		for _, redirect := range old.Redirects {
			if err := os.RemoveAll(filepath.Join(stage, filepath.FromSlash(strings.TrimPrefix(redirect, "/")))); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"assets", "impressum", "datenschutzerklaerung"} {
		if err := copyTree(filepath.Join(static, name), filepath.Join(stage, name)); err != nil {
			return fmt.Errorf("copy shared %s: %w", name, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(stage, "assets", "posts")); err != nil {
		return err
	}
	for _, asset := range graph.Assets {
		if err := copyFile(asset.SourcePath, filepath.Join(stage, "assets", "posts", asset.Filename)); err != nil {
			return err
		}
	}
	for _, value := range posts {
		if !renderAll {
			if old, found := previous.Posts[value.Slug]; found && old.Fingerprint == current[value.Slug].Fingerprint {
				continue
			}
		}
		rendered := cloneForRender(value, graph.Links[value.Path])
		html, err := render.Post(rendered)
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(stage, value.Slug, "index.html"), []byte(html)); err != nil {
			return err
		}
	}
	home, err := render.Home(posts)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(stage, "index.html"), []byte(home)); err != nil {
		return err
	}
	for _, value := range posts {
		for _, redirect := range value.Redirects {
			redirectPath := filepath.Join(stage, filepath.FromSlash(strings.TrimPrefix(redirect, "/")), "index.html")
			page := `<!doctype html><meta http-equiv="refresh" content="0; url=/` + value.Slug + `/"><link rel="canonical" href="/` + value.Slug + `/">`
			if err := writeFile(redirectPath, []byte(page)); err != nil {
				return err
			}
		}
	}
	if err := writeFile(filepath.Join(stage, "robots.txt"), []byte("User-agent: *\nAllow: /\nSitemap: https://engineering.paulheymann.de/sitemap.xml\n")); err != nil {
		return err
	}
	return writeFile(filepath.Join(stage, "sitemap.xml"), []byte(sitemap(posts)))
}

func postStates(posts []*post.Post) map[string]postState {
	states := make(map[string]postState, len(posts))
	for _, value := range posts {
		states[value.Slug] = postState{Fingerprint: postFingerprint(value), Slug: value.Slug, Redirects: append([]string(nil), value.Redirects...)}
	}
	return states
}

func postFingerprint(value *post.Post) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\000%s\000%s\000%s\000%s\000", value.Path, value.Title, value.Date.Format("2006-01-02"), value.Body, strings.Join(value.Redirects, "\000"))
	return hex.EncodeToString(hash.Sum(nil))
}

func cloneForRender(value *post.Post, links []contentgraph.Link) *post.Post {
	clone := *value
	clone.Body = obsidianImage.ReplaceAllStringFunc(clone.Body, func(match string) string {
		parts := obsidianImage.FindStringSubmatch(match)
		filename := filepath.Base(strings.TrimSpace(parts[1]))
		alt := strings.TrimSuffix(filename, filepath.Ext(filename))
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			alt = strings.TrimSpace(parts[2])
		}
		return "![" + alt + "](/assets/posts/" + url.PathEscape(filename) + ")"
	})
	index := 0
	clone.Body = wikilink.ReplaceAllStringFunc(clone.Body, func(match string) string {
		if index >= len(links) {
			return match
		}
		link := links[index]
		index++
		parts := strings.SplitN(strings.TrimSuffix(strings.TrimPrefix(match, "[["), "]]"), "|", 2)
		label := parts[0]
		if len(parts) == 2 {
			label = parts[1]
		}
		return "[" + label + "](" + link.URL + ")"
	})
	clone.Body = image.ReplaceAllStringFunc(clone.Body, func(match string) string {
		parts := image.FindStringSubmatch(match)
		return parts[1] + "/assets/posts/" + filepath.Base(parts[2])
	})
	return &clone
}

func sitemap(posts []*post.Post) string {
	var output strings.Builder
	output.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n  <url><loc>https://engineering.paulheymann.de/</loc></url>\n")
	ordered := append([]*post.Post(nil), posts...)
	sort.Slice(ordered, func(first, second int) bool { return ordered[first].Slug < ordered[second].Slug })
	for _, value := range ordered {
		fmt.Fprintf(&output, "  <url><loc>https://engineering.paulheymann.de/%s/</loc><lastmod>%s</lastmod></url>\n", value.Slug, value.Date.Format("2006-01-02"))
	}
	output.WriteString("  <url><loc>https://engineering.paulheymann.de/impressum/</loc></url>\n  <url><loc>https://engineering.paulheymann.de/datenschutzerklaerung/</loc></url>\n</urlset>\n")
	return output.String()
}

func fingerprintFor(posts []*post.Post, graph *contentgraph.Graph, static string) (string, error) {
	hash := sha256.New()
	for _, value := range posts {
		fmt.Fprint(hash, postFingerprint(value), "\000")
	}
	for _, asset := range graph.Assets {
		if err := hashFile(hash, asset.SourcePath); err != nil {
			return "", err
		}
	}
	if err := hashTree(hash, static); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashTree(hash io.Writer, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if _, err := io.WriteString(hash, path); err != nil {
			return err
		}
		return hashFile(hash, path)
	})
}
func hashFile(hash io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(hash, file)
	return err
}
func fileManifest(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		if err := hashFile(hash, path); err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = hex.EncodeToString(hash.Sum(nil))
		return nil
	})
	return files, err
}
func outputMatches(root string, expected map[string]string) bool {
	if len(expected) == 0 || !directoryExists(root) {
		return false
	}
	actual, err := fileManifest(root)
	if err != nil || len(actual) != len(expected) {
		return false
	}
	for path, hash := range expected {
		if actual[path] != hash {
			return false
		}
	}
	return true
}
func readState(path string) (buildState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return buildState{}, err
	}
	var state buildState
	err = json.Unmarshal(data, &state)
	return state, err
}
func writeState(path string, state buildState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}
func directoryExists(path string) bool { info, err := os.Stat(path); return err == nil && info.IsDir() }
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func hardlinkTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if err := os.Link(path, destination); err == nil {
			return nil
		}
		return copyFile(path, destination)
	})
}
func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, relative), 0o755)
		}
		return copyFile(path, filepath.Join(target, relative))
	})
}
func replaceDirectory(stage, output string) error {
	backup := output + ".previous"
	_ = os.RemoveAll(backup)
	if directoryExists(output) {
		if err := os.Rename(output, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, output); err != nil {
		if directoryExists(backup) {
			_ = os.Rename(backup, output)
		}
		return err
	}
	return os.RemoveAll(backup)
}
