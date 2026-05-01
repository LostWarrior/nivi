package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadIncludesAllMatchingFilesInDeterministicOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	project := filepath.Join(workspace, "project")

	mustMkdirAll(t, filepath.Join(root, ".nivi"))
	mustMkdirAll(t, filepath.Join(workspace, ".nivi"))
	mustMkdirAll(t, project)

	rootNivi := filepath.Join(root, "NIVI.md")
	rootHiddenAgent := filepath.Join(root, ".nivi", "agent.md")
	workspaceAgent := filepath.Join(workspace, "agent.md")
	workspaceHiddenClaude := filepath.Join(workspace, ".nivi", "CLAUDE.md")
	projectClaude := filepath.Join(project, "claude.md")

	mustWriteFile(t, rootNivi, "root instructions")
	mustWriteFile(t, rootHiddenAgent, "root hidden agent")
	mustWriteFile(t, workspaceAgent, "workspace agent")
	mustWriteFile(t, workspaceHiddenClaude, "workspace hidden claude")
	mustWriteFile(t, projectClaude, "project claude")

	loaded, err := Load(project)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantPaths := []string{
		rootNivi,
		rootHiddenAgent,
		workspaceAgent,
		workspaceHiddenClaude,
		projectClaude,
	}
	if len(loaded.Paths) != len(wantPaths) {
		t.Fatalf("paths length = %d, want %d; paths=%v", len(loaded.Paths), len(wantPaths), loaded.Paths)
	}
	for index := range wantPaths {
		if normalizePath(loaded.Paths[index]) != normalizePath(wantPaths[index]) {
			t.Fatalf("paths[%d] = %q, want %q", index, loaded.Paths[index], wantPaths[index])
		}
	}

	mustContainInOrder(t, loaded.Content,
		"File: "+loaded.Paths[0]+"\nroot instructions",
		"File: "+loaded.Paths[1]+"\nroot hidden agent",
		"File: "+loaded.Paths[2]+"\nworkspace agent",
		"File: "+loaded.Paths[3]+"\nworkspace hidden claude",
		"File: "+loaded.Paths[4]+"\nproject claude",
	)
}

func TestLoadSupportsAgentAndClaudeNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".nivi"))

	agentPath := filepath.Join(root, "agent.md")
	claudePath := filepath.Join(root, "claude.md")
	hiddenAgentPath := filepath.Join(root, ".nivi", "AGENT.md")
	hiddenClaudePath := filepath.Join(root, ".nivi", "claude.md")

	mustWriteFile(t, agentPath, "agent")
	mustWriteFile(t, claudePath, "claude")
	mustWriteFile(t, hiddenAgentPath, "hidden-agent")
	mustWriteFile(t, hiddenClaudePath, "hidden-claude")

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantPaths := []string{
		agentPath,
		claudePath,
		hiddenAgentPath,
		hiddenClaudePath,
	}
	if len(loaded.Paths) != len(wantPaths) {
		t.Fatalf("paths length = %d, want %d; paths=%v", len(loaded.Paths), len(wantPaths), loaded.Paths)
	}
	for index := range wantPaths {
		if normalizePath(loaded.Paths[index]) != normalizePath(wantPaths[index]) {
			t.Fatalf("paths[%d] = %q, want %q", index, loaded.Paths[index], wantPaths[index])
		}
	}
}

func TestExistingPathsDeduplicatesCaseInsensitiveAliases(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	upperPath := filepath.Join(root, "NIVI.md")
	mustWriteFile(t, upperPath, "nivi")

	if _, err := os.Stat(filepath.Join(root, "nivi.md")); err != nil {
		t.Skip("case-sensitive filesystem; skipping alias-deduplication check")
	}

	paths, err := existingPaths(root)
	if err != nil {
		t.Fatalf("existingPaths() error = %v", err)
	}

	matches := 0
	for _, path := range paths {
		if strings.EqualFold(filepath.Base(path), "nivi.md") {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("nivi alias matches = %d, want 1; paths=%v", matches, paths)
	}
}

func mustContainInOrder(t *testing.T, body string, parts ...string) {
	t.Helper()

	position := 0
	for _, part := range parts {
		next := strings.Index(body[position:], part)
		if next < 0 {
			t.Fatalf("missing %q in %q", part, body)
		}
		position += next + len(part)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func normalizePath(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
