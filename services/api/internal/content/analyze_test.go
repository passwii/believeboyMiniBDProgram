package content

import (
	"errors"
	"slices"
	"testing"
)

func TestAnalyzeExtractsSelfContainedMetadata(t *testing.T) {
	data := []byte(`<!doctype html>
<html>
<head>
  <title> 客户方案 A </title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>.logo { background: url(data:image/png;base64,AAAA); }</style>
</head>
<body>
  <img src="data:image/png;base64,BBBB">
  <script>window.answer = 42;</script>
</body>
</html>`)

	analysis, err := Analyze(data, "proposal.html")
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if analysis.Title != "客户方案 A" {
		t.Fatalf("Title = %q", analysis.Title)
	}
	if !analysis.ViewportPresent {
		t.Fatal("ViewportPresent = false")
	}
	if analysis.ScriptCount != 1 {
		t.Fatalf("ScriptCount = %d", analysis.ScriptCount)
	}
	if len(analysis.ExternalResources) != 0 {
		t.Fatalf("ExternalResources = %v", analysis.ExternalResources)
	}
	if len(analysis.Warnings) != 0 {
		t.Fatalf("Warnings = %v", analysis.Warnings)
	}
}

func TestAnalyzeFallsBackToFilenameAndWarnsAboutViewport(t *testing.T) {
	analysis, err := Analyze([]byte(`<html><body><p>hello</p></body></html>`), `My Proposal.html`)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if analysis.Title != "My Proposal" {
		t.Fatalf("Title = %q", analysis.Title)
	}
	if analysis.ViewportPresent {
		t.Fatal("ViewportPresent = true")
	}
	if len(analysis.Warnings) != 2 {
		t.Fatalf("Warnings = %v", analysis.Warnings)
	}
}

func TestAnalyzeRejectsExternalResources(t *testing.T) {
	data := []byte(`<!doctype html>
<html>
<head>
  <title>External</title>
  <script src="https://cdn.example/app.js"></script>
  <style>.hero { background-image: url('https://cdn.example/hero.png'); }</style>
</head>
<body>
  <img src="//images.example/photo.png">
  <div style="background:url(https://cdn.example/again.png)"></div>
</body>
</html>`)

	analysis, err := Analyze(data, "external.html")
	if !errors.Is(err, ErrExternalResources) {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := []string{
		"https://cdn.example/app.js",
		"https://cdn.example/hero.png",
		"//images.example/photo.png",
		"https://cdn.example/again.png",
	}
	if !slices.Equal(analysis.ExternalResources, want) {
		t.Fatalf("ExternalResources = %v, want %v", analysis.ExternalResources, want)
	}
}

func TestAnalyzeAllowsNonResourceExternalLinks(t *testing.T) {
	data := []byte(`<html><head><title>Links</title></head><body>
<a href="https://example.com/help">help</a>
</body></html>`)

	analysis, err := Analyze(data, "links.html")
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(analysis.ExternalResources) != 0 {
		t.Fatalf("ExternalResources = %v", analysis.ExternalResources)
	}
}

func TestAnalyzeRejectsEmptyInput(t *testing.T) {
	_, err := Analyze([]byte(" \n\t"), "empty.html")
	if !errors.Is(err, ErrInvalidHTML) {
		t.Fatalf("Analyze() error = %v", err)
	}
}
