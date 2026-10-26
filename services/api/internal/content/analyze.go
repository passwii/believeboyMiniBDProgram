package content

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	ErrInvalidHTML       = errors.New("invalid HTML document")
	ErrExternalResources = errors.New("external resources are not allowed")
	styleURLPattern      = regexp.MustCompile(`(?i)url\(\s*['"]?([^'")\s]+)['"]?\s*\)`)
)

type Analysis struct {
	Title             string
	ViewportPresent   bool
	ScriptCount       int
	ExternalResources []string
	Warnings          []string
}

func Analyze(data []byte, filename string) (Analysis, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Analysis{}, ErrInvalidHTML
	}
	document, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return Analysis{}, fmt.Errorf("%w: %v", ErrInvalidHTML, err)
	}

	analysis := Analysis{
		ViewportPresent:   false,
		ExternalResources: []string{},
		Warnings:          []string{},
	}
	seenResources := make(map[string]struct{})
	var titleText string

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			attrs := attributes(node)

			switch tag {
			case "title":
				if titleText == "" {
					titleText = strings.TrimSpace(nodeText(node))
				}
			case "meta":
				if strings.EqualFold(attrs["name"], "viewport") && strings.TrimSpace(attrs["content"]) != "" {
					analysis.ViewportPresent = true
				}
			case "script":
				analysis.ScriptCount++
			}

			for _, candidate := range resourceValues(tag, attrs) {
				if !isExternalResource(candidate) {
					continue
				}
				if _, exists := seenResources[candidate]; exists {
					continue
				}
				seenResources[candidate] = struct{}{}
				analysis.ExternalResources = append(analysis.ExternalResources, candidate)
			}
			inspectCSSURLs(attrs["style"], &analysis, seenResources)
		}
		if node.Type == html.TextNode && node.Parent != nil && strings.EqualFold(node.Parent.Data, "style") {
			inspectCSSURLs(node.Data, &analysis, seenResources)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	analysis.Title = strings.TrimSpace(titleText)
	if analysis.Title == "" {
		analysis.Title = filenameStem(filename)
		analysis.Warnings = append(analysis.Warnings, "HTML 缺少 title，已使用文件名作为方案标题")
	}
	if !analysis.ViewportPresent {
		analysis.Warnings = append(analysis.Warnings, "HTML 缺少 viewport 声明，移动端和宽屏适配可能异常")
	}
	if len(analysis.ExternalResources) > 0 {
		return analysis, fmt.Errorf("%w: %d found", ErrExternalResources, len(analysis.ExternalResources))
	}
	return analysis, nil
}

func attributes(node *html.Node) map[string]string {
	values := make(map[string]string, len(node.Attr))
	for _, attr := range node.Attr {
		values[strings.ToLower(attr.Key)] = strings.TrimSpace(attr.Val)
	}
	return values
}

func resourceValues(tag string, attrs map[string]string) []string {
	values := make([]string, 0, 3)
	appendAttr := func(name string) {
		if value := attrs[name]; value != "" {
			values = append(values, value)
		}
	}

	switch tag {
	case "script", "img", "iframe", "source", "video", "audio", "track", "embed":
		appendAttr("src")
	case "link":
		appendAttr("href")
	}

	if tag == "img" || tag == "source" {
		if srcset := attrs["srcset"]; srcset != "" {
			for _, item := range strings.Split(srcset, ",") {
				fields := strings.Fields(strings.TrimSpace(item))
				if len(fields) > 0 {
					values = append(values, fields[0])
				}
			}
		}
	}
	if tag == "video" {
		appendAttr("poster")
	}
	return values
}

func inspectCSSURLs(value string, analysis *Analysis, seen map[string]struct{}) {
	for _, match := range styleURLPattern.FindAllStringSubmatch(value, -1) {
		if len(match) < 2 || !isExternalResource(match[1]) {
			continue
		}
		if _, exists := seen[match[1]]; exists {
			continue
		}
		seen[match[1]] = struct{}{}
		analysis.ExternalResources = append(analysis.ExternalResources, match[1])
	}
}

func isExternalResource(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func filenameStem(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" || stem == "." {
		return "未命名方案"
	}
	return stem
}
