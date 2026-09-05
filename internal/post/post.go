// Package post parses vault Markdown into the preview pipeline's post model.
package post

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	firstH1Pattern = regexp.MustCompile(`^#\s+(.+?)\s*$`)
	tagPattern     = regexp.MustCompile(`(?:^|[^[:alnum:]_/-])#([[:alnum:]_/-]+)`)
	tagOnlyPattern = regexp.MustCompile(`^\s*(?:#[[:alnum:]_/-]+\s*)+$`)
)

// Post is the parsed source representation of a previewable vault post.
type Post struct {
	Path      string
	Title     string
	Date      time.Time
	Redirects []string
	Tags      []string
	Slug      string
	Excerpt   string
	Body      string
}

// Parse returns a post when Markdown is tagged #blog, #engineering, and
// #preview. Other Markdown is intentionally ignored and returns (nil, nil).
func Parse(filePath, markdown string) (*Post, error) {
	frontMatter, body, err := splitFrontMatter(markdown)
	if err != nil {
		return nil, pathError(filePath, err)
	}

	tags := tagsIn(body)
	if !isPreviewPost(tags) {
		return nil, nil
	}

	date, redirects, err := metadata(frontMatter)
	if err != nil {
		return nil, pathError(filePath, err)
	}

	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	postBody := contentBody(body, title)
	slug := Slug(title)
	if slug == "" {
		return nil, pathError(filePath, fmt.Errorf("title %q does not produce a usable slug", title))
	}

	return &Post{
		Path:      filePath,
		Title:     title,
		Date:      date,
		Redirects: redirects,
		Tags:      tags,
		Slug:      slug,
		Excerpt:   firstTextLines(postBody, 5),
		Body:      postBody,
	}, nil
}

// Slug creates a flat URL slug from a title. Letters and digits are retained;
// all other runs become one hyphen.
func Slug(title string) string {
	var result strings.Builder
	separatorPending := false
	for _, character := range strings.ToLower(title) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if separatorPending && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(character)
			separatorPending = false
		} else if result.Len() > 0 {
			separatorPending = true
		}
	}
	return result.String()
}

func splitFrontMatter(markdown string) ([]string, string, error) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, markdown, nil
	}
	for index := 1; index < len(lines); index++ {
		if lines[index] == "---" {
			return lines[1:index], strings.Join(lines[index+1:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("front matter is not terminated")
}

func metadata(lines []string) (time.Time, []string, error) {
	var dateValue string
	redirects := make([]string, 0)
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "date:") {
			if dateValue != "" {
				return time.Time{}, nil, fmt.Errorf("date property is repeated")
			}
			dateValue = strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			continue
		}
		if !strings.HasPrefix(line, "redirect:") {
			continue
		}
		if strings.TrimSpace(strings.TrimPrefix(line, "redirect:")) != "" {
			return time.Time{}, nil, fmt.Errorf("redirect property must be a list")
		}
		for index++; index < len(lines); index++ {
			item := strings.TrimSpace(lines[index])
			if !strings.HasPrefix(item, "-") {
				index--
				break
			}
			redirect := strings.TrimSpace(strings.TrimPrefix(item, "-"))
			if err := validRedirect(redirect); err != nil {
				return time.Time{}, nil, err
			}
			if contains(redirects, redirect) {
				return time.Time{}, nil, fmt.Errorf("redirect %q is repeated", redirect)
			}
			redirects = append(redirects, redirect)
		}
	}
	if dateValue == "" {
		return time.Time{}, nil, fmt.Errorf("date property is required")
	}
	date, err := time.Parse("2006-01-02", dateValue)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("date property must use YYYY-MM-DD: %w", err)
	}
	return date, redirects, nil
}

func validRedirect(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("redirect %q must be an absolute site path", value)
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("redirect %q must not contain path traversal", value)
		}
	}
	return nil
}

func contentBody(body, title string) string {
	lines := strings.Split(body, "\n")
	content := make([]string, 0, len(lines))
	for _, line := range lines {
		if tagOnlyPattern.MatchString(line) {
			continue
		}
		matches := firstH1Pattern.FindStringSubmatch(line)
		if matches != nil && strings.TrimSpace(strings.TrimSuffix(matches[1], "#")) == title {
			continue
		}
		content = append(content, line)
	}
	return strings.TrimSpace(strings.Join(content, "\n"))
}

func tagsIn(body string) []string {
	matches := tagPattern.FindAllStringSubmatch(body, -1)
	tags := make([]string, 0, len(matches))
	for _, match := range matches {
		tag := match[1]
		if !contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}

func isPreviewPost(tags []string) bool {
	return contains(tags, "blog") && contains(tags, "engineering") && contains(tags, "preview")
}

func firstTextLines(body string, count int) string {
	selected := make([]string, 0, count)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isImageOnly(line) {
			continue
		}
		selected = append(selected, line)
		if len(selected) == count {
			break
		}
	}
	return strings.Join(selected, "\n")
}

func isImageOnly(line string) bool {
	return (strings.HasPrefix(line, "![[") && strings.HasSuffix(line, "]]")) ||
		(strings.HasPrefix(line, "![") && strings.Contains(line, "](") && strings.HasSuffix(line, ")"))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func pathError(filePath string, err error) error {
	return fmt.Errorf("%s: %w", filePath, err)
}
