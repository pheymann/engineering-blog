// Package post parses vault Markdown into the preview pipeline's post model.
package post

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	firstH1Pattern = regexp.MustCompile(`^#\s+(.+?)\s*$`)
	tagPattern     = regexp.MustCompile(`(?:^|[^[:alnum:]_/-])#([[:alnum:]_/-]+)`)
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

	title, postBody, err := titleAndBody(body)
	if err != nil {
		return nil, pathError(filePath, err)
	}
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
		Excerpt:   firstLines(postBody, 5),
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

func titleAndBody(body string) (string, string, error) {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		matches := firstH1Pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		title := strings.TrimSpace(strings.TrimSuffix(matches[1], "#"))
		if title == "" {
			return "", "", fmt.Errorf("first H1 title is empty")
		}
		return title, strings.Join(lines[index+1:], "\n"), nil
	}
	return "", "", fmt.Errorf("first H1 title is required")
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

func firstLines(body string, count int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > count {
		lines = lines[:count]
	}
	return strings.Join(lines, "\n")
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
