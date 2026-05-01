package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSandboxPathInsideRoot(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	absPath, rel, err := resolveSandboxPath(root, "nested")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if absPath != targetDir {
		t.Fatalf("expected %s, got %s", targetDir, absPath)
	}
	if rel != "nested" {
		t.Fatalf("expected nested rel path, got %s", rel)
	}
}

func TestResolveSandboxPathBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	if _, _, err := resolveSandboxPath(root, "../outside"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestToolApplyPatchSingleReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("alpha beta alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := toolApplyPatch(root, applyPatchArgs{
		Path:    "file.txt",
		OldText: "alpha",
		NewText: "gamma",
	})
	if err != nil {
		t.Fatalf("expected patch apply to succeed, got %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "gamma beta alpha" {
		t.Fatalf("expected only first match replaced, got %q", string(content))
	}
}
