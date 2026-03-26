package api

import (
	"strings"
	"testing"
)

func TestBuildItemContentURL(t *testing.T) {
	url := buildItemContentURL(
		"https://dev.azure.com/yoizen",
		"yFlow",
		"repo name",
		"/src/app.go",
		"abc123",
	)

	if !strings.HasPrefix(url, "https://dev.azure.com/yoizen/yFlow/_apis/git/repositories/repo%20name/items?") {
		t.Fatalf("unexpected url prefix: %s", url)
	}
	if !strings.Contains(url, "path=%2Fsrc%2Fapp.go") {
		t.Fatalf("expected encoded path in url: %s", url)
	}
	if !strings.Contains(url, "versionDescriptor.version=abc123") {
		t.Fatalf("expected version in url: %s", url)
	}
	if !strings.Contains(url, "versionDescriptor.versionType=commit") {
		t.Fatalf("expected version type in url: %s", url)
	}
}

func TestCountUnifiedDiffStats(t *testing.T) {
	diff := strings.Join([]string{
		"--- old.txt",
		"+++ new.txt",
		"@@ -1,3 +1,3 @@",
		" line 1",
		"-line 2",
		"+line 2 updated",
		" line 3",
	}, "\n")

	additions, deletions := countUnifiedDiffStats(diff)
	if additions != 1 {
		t.Fatalf("expected 1 addition, got %d", additions)
	}
	if deletions != 1 {
		t.Fatalf("expected 1 deletion, got %d", deletions)
	}
}

func TestRenderUnifiedDiff(t *testing.T) {
	diff, additions, deletions, err := renderUnifiedDiff("old.txt", "new.txt", "line 1\nline 2\n", "line 1\nline 2 updated\n")
	if err != nil {
		t.Fatalf("renderUnifiedDiff returned error: %v", err)
	}
	if additions != 1 {
		t.Fatalf("expected 1 addition, got %d", additions)
	}
	if deletions != 1 {
		t.Fatalf("expected 1 deletion, got %d", deletions)
	}
	if !strings.Contains(diff, "--- old.txt") || !strings.Contains(diff, "+++ new.txt") {
		t.Fatalf("unexpected diff output: %s", diff)
	}
	if !strings.Contains(diff, "-line 2") || !strings.Contains(diff, "+line 2 updated") {
		t.Fatalf("expected unified diff hunks in output: %s", diff)
	}
}
