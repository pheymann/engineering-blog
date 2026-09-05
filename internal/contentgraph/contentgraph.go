// Package contentgraph validates relationships between parsed vault posts.
package contentgraph

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pheymann/engineering-blog/internal/post"
)

var (
	wikilinkPattern      = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	imagePattern         = regexp.MustCompile(`!\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)`)
	obsidianImagePattern = regexp.MustCompile(`!\[\[([^\]|]+)(?:\|[^\]]*)?\]\]`)
	calloutPattern       = regexp.MustCompile(`(?mi)^\s*>\s*\[![^\]]+\]`)
	excalidrawPattern    = regexp.MustCompile(`(?i)excalidraw-plugin:|\[\[[^\]]*\.excalidraw(?:\.md)?(?:\|[^\]]*)?\]\]`)
)

// Graph contains resolved relationships without changing post bodies.
type Graph struct {
	Posts  []*post.Post
	Links  map[string][]Link
	Assets []Asset
}

// Link maps one source wikilink to its generated post URL.
type Link struct {
	Reference string
	URL       string
	Target    *post.Post
}

// Asset is a vault image required by one or more posts. Filename is the name
// used by the later output copier, so duplicate names are rejected here.
type Asset struct {
	SourcePath string
	Filename   string
}

// Validate builds a graph for preview posts and rejects relationships that
// would make a generated preview ambiguous or incomplete.
func Validate(vaultRoot string, posts []*post.Post) (*Graph, error) {
	root, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	root = filepath.Clean(root)

	graph := &Graph{Posts: posts, Links: make(map[string][]Link)}
	assets, err := indexAssets(root)
	if err != nil {
		return nil, err
	}
	bySlug := make(map[string]*post.Post, len(posts))
	byReference := make(map[string]*post.Post, len(posts)*3)
	urlOwners := make(map[string]*post.Post, len(posts)*2)
	for _, current := range posts {
		if current == nil {
			return nil, fmt.Errorf("cannot validate a nil post")
		}
		if previous, found := bySlug[current.Slug]; found {
			return nil, fmt.Errorf("%s: slug %q conflicts with %s", current.Path, current.Slug, previous.Path)
		}
		bySlug[current.Slug] = current
		for _, reference := range postReferences(current) {
			if previous, found := byReference[reference]; found && previous != current {
				return nil, fmt.Errorf("%s: post reference %q conflicts with %s", current.Path, reference, previous.Path)
			}
			byReference[reference] = current
		}
		if err := claimURL(urlOwners, currentURL(current), current, "current URL"); err != nil {
			return nil, err
		}
	}
	for _, current := range posts {
		for _, redirect := range current.Redirects {
			if err := claimURL(urlOwners, redirect, current, "redirect"); err != nil {
				return nil, err
			}
		}
	}

	assetsByFilename := make(map[string]Asset)
	for _, current := range posts {
		if err := rejectUnsupportedSyntax(current); err != nil {
			return nil, err
		}
		links, err := resolveLinks(current, byReference)
		if err != nil {
			return nil, err
		}
		graph.Links[current.Path] = links

		for _, imageReference := range imageReferences(current.Body) {
			asset, err := resolveAsset(root, assets, current, imageReference)
			if err != nil {
				return nil, err
			}
			if previous, found := assetsByFilename[asset.Filename]; found && previous.SourcePath != asset.SourcePath {
				return nil, fmt.Errorf("%s: asset filename %q conflicts with %s", current.Path, asset.Filename, previous.SourcePath)
			}
			assetsByFilename[asset.Filename] = asset
		}
	}
	for _, asset := range assetsByFilename {
		graph.Assets = append(graph.Assets, asset)
	}
	sort.Slice(graph.Assets, func(first, second int) bool {
		return graph.Assets[first].SourcePath < graph.Assets[second].SourcePath
	})
	return graph, nil
}

func postReferences(current *post.Post) []string {
	base := strings.TrimSuffix(filepath.Base(current.Path), filepath.Ext(current.Path))
	return []string{current.Title, current.Slug, base}
}

func currentURL(current *post.Post) string {
	return "/" + current.Slug + "/"
}

func claimURL(owners map[string]*post.Post, value string, owner *post.Post, kind string) error {
	route := normalizedRoute(value)
	if previous, found := owners[route]; found {
		return fmt.Errorf("%s: %s %q conflicts with %s", owner.Path, kind, value, previous.Path)
	}
	owners[route] = owner
	return nil
}

// normalizedRoute models the static server's path resolution, where trailing
// and repeated slashes do not create distinct generated directories.
func normalizedRoute(value string) string {
	return path.Clean("/" + strings.TrimPrefix(value, "/"))
}

func rejectUnsupportedSyntax(current *post.Post) error {
	switch {
	case calloutPattern.MatchString(current.Body):
		return fmt.Errorf("%s: Obsidian callouts are not supported", current.Path)
	case excalidrawPattern.MatchString(current.Body):
		return fmt.Errorf("%s: Excalidraw content is not supported", current.Path)
	}
	return nil
}

func resolveLinks(current *post.Post, posts map[string]*post.Post) ([]Link, error) {
	bodyWithoutImages := obsidianImagePattern.ReplaceAllString(current.Body, "")
	matches := wikilinkPattern.FindAllStringSubmatch(bodyWithoutImages, -1)
	links := make([]Link, 0, len(matches))
	for _, match := range matches {
		reference := strings.TrimSpace(strings.SplitN(match[1], "|", 2)[0])
		target, found := posts[reference]
		if !found {
			return nil, fmt.Errorf("%s: broken post link %q", current.Path, reference)
		}
		links = append(links, Link{Reference: reference, URL: currentURL(target), Target: target})
	}
	return links, nil
}

func imageReferences(body string) []string {
	matches := imagePattern.FindAllStringSubmatch(body, -1)
	references := make([]string, 0, len(matches))
	for _, match := range matches {
		references = append(references, match[1])
	}
	for _, match := range obsidianImagePattern.FindAllStringSubmatch(body, -1) {
		references = append(references, strings.TrimSpace(match[1]))
	}
	return references
}

func resolveAsset(root string, assets assetIndex, current *post.Post, reference string) (Asset, error) {
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" {
		return Asset{}, fmt.Errorf("%s: image %q must be a local vault path", current.Path, reference)
	}
	sourcePost, err := insideRoot(root, current.Path)
	if err != nil {
		return Asset{}, fmt.Errorf("%s: %w", current.Path, err)
	}
	assetPath, err := imagePath(root, sourcePost, parsed.Path)
	if err != nil {
		return Asset{}, fmt.Errorf("%s: image %q escapes the vault", current.Path, reference)
	}
	if info, err := os.Stat(assetPath); err == nil {
		if !info.Mode().IsRegular() {
			return Asset{}, fmt.Errorf("%s: image %q is not a file", current.Path, reference)
		}
		return Asset{SourcePath: assetPath, Filename: filepath.Base(assetPath)}, nil
	}

	if strings.Contains(parsed.Path, "/") {
		return Asset{}, fmt.Errorf("%s: image %q is missing", current.Path, reference)
	}
	candidates := assets.byFilename[filepath.Base(parsed.Path)]
	switch len(candidates) {
	case 0:
		return Asset{}, fmt.Errorf("%s: image %q is missing", current.Path, reference)
	case 1:
		return Asset{SourcePath: candidates[0], Filename: filepath.Base(candidates[0])}, nil
	default:
		return Asset{}, fmt.Errorf("%s: image %q is ambiguous between %s", current.Path, reference, strings.Join(candidates, ", "))
	}
}

func imagePath(root, sourcePost, reference string) (string, error) {
	if strings.HasPrefix(reference, "/") {
		return insideRoot(root, filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(reference, "/"))))
	}
	return insideRoot(root, filepath.Join(filepath.Dir(sourcePost), filepath.FromSlash(reference)))
}

type assetIndex struct {
	byFilename map[string][]string
}

func indexAssets(root string) (assetIndex, error) {
	index := assetIndex{byFilename: make(map[string][]string)}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		index.byFilename[entry.Name()] = append(index.byFilename[entry.Name()], path)
		return nil
	})
	if err != nil {
		return assetIndex{}, fmt.Errorf("scan vault assets: %w", err)
	}
	for filename := range index.byFilename {
		sort.Strings(index.byFilename[filename])
	}
	return index, nil
}

func insideRoot(root, candidate string) (string, error) {
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the vault", candidate)
	}
	return candidate, nil
}
