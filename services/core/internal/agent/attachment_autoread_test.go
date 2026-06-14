package agent

import (
	"testing"
)

func TestExtractAttachmentIDs(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single attachment",
			text:     "User attached file(s) to this session. Use the `attach_read` tool with the matching id to view content.\n\n- id=1dab1fdf-923c-473f-9294-dc8336d8a65a · Screenshot.png · image/png · 2212084 bytes\n",
			expected: []string{"1dab1fdf-923c-473f-9294-dc8336d8a65a"},
		},
		{
			name: "multiple attachments",
			text: `User attached file(s) to this session. Use the ` + "`attach_read`" + ` tool with the matching id to view content.

- id=aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee · file1.txt · text/plain · 100 bytes
- id=11111111-2222-3333-4444-555555555555 · file2.pdf · application/pdf · 5000 bytes
`,
			expected: []string{
				"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"11111111-2222-3333-4444-555555555555",
			},
		},
		{
			name:     "no attachments",
			text:     "User asked a question about code",
			expected: []string{},
		},
		{
			name:     "empty string",
			text:     "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAttachmentIDs(tt.text)
			if len(got) != len(tt.expected) {
				t.Errorf("extractAttachmentIDs() returned %d IDs, want %d", len(got), len(tt.expected))
				return
			}
			for i, id := range got {
				if id != tt.expected[i] {
					t.Errorf("extractAttachmentIDs()[%d] = %q, want %q", i, id, tt.expected[i])
				}
			}
		})
	}
}

func TestIsAttachmentSystemMessage(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "attachment system message",
			text:     "User attached file(s) to this session. Use the `attach_read` tool with the matching id to view content.\n\n- id=1dab1fdf-923c-473f-9294-dc8336d8a65a · Screenshot.png",
			expected: true,
		},
		{
			name:     "regular system message",
			text:     "User started a new session",
			expected: false,
		},
		{
			name:     "empty string",
			text:     "",
			expected: false,
		},
		{
			name:     "mentions attach_read but no id",
			text:     "Use the attach_read tool to read attachments",
			expected: false,
		},
		{
			name:     "mentions id but no attach_read",
			text:     "id=12345678-1234-1234-1234-123456789012",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAttachmentSystemMessage(tt.text)
			if got != tt.expected {
				t.Errorf("isAttachmentSystemMessage() = %v, want %v", got, tt.expected)
			}
		})
	}
}
