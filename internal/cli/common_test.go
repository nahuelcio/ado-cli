package cli

import (
	"testing"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "string shorter than maxLen",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "string equal to maxLen",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "string longer than maxLen",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen less than 3 returns truncated with ellipsis",
			input:    "hello",
			maxLen:   2,
			expected: "he...",
		},
		{
			name:     "maxLen is 3 returns just ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no HTML tags",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "br tag",
			input:    "hello<br>world",
			expected: "hello\nworld",
		},
		{
			name:     "br tags multiple",
			input:    "line1<br>line2<br/>line3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "div tag",
			input:    "<div>content</div>",
			expected: "content",
		},
		{
			name:     "p tag",
			input:    "<p>paragraph</p>",
			expected: "paragraph",
		},
		{
			name:     "nested tags",
			input:    "<div><p>text</p></div>",
			expected: "text",
		},
		{
			name:     "nbsp entity",
			input:    "hello&nbsp;world",
			expected: "hello world",
		},
		{
			name:     "lt gt entities",
			input:    "&lt;div&gt;",
			expected: "<div>",
		},
		{
			name:     "amp entity",
			input:    "foo &amp; bar",
			expected: "foo & bar",
		},
		{
			name:     "multiple newlines",
			input:    "a\n\n\nb",
			expected: "a\n\nb",
		},
		{
			name:     "mixed content",
			input:    "<p>Hello</p><br><div>World</div>",
			expected: "Hello\n\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
