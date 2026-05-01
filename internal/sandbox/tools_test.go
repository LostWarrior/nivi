package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewToolsetRejectsEmptyRoot(t *testing.T) {
	t.Parallel()

	_, err := NewToolset(" ")
	if err == nil {
		t.Fatal("NewToolset() expected error for empty root")
	}
}

func TestPWDReturnsResolvedRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	if got, want := toolset.PWD(), toolset.Root(); got != want {
		t.Fatalf("PWD() = %q, want %q", got, want)
	}
}

func TestLSListsDirectoryAndRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.txt"), "a")
	mustMkdirAll(t, filepath.Join(root, "sub"))

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	output, err := toolset.LS(".")
	if err != nil {
		t.Fatalf("LS() error = %v", err)
	}
	if !strings.Contains(output, "a.txt") {
		t.Fatalf("LS output missing file: %q", output)
	}
	if !strings.Contains(output, "sub/") {
		t.Fatalf("LS output missing directory: %q", output)
	}

	if _, err := toolset.LS("../"); err == nil {
		t.Fatal("LS() expected traversal error")
	}
}

func TestReadFileSupportsUTF8AndTruncates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	content := strings.Repeat("x", defaultMaxReadBytes+128)
	mustWriteFile(t, path, content)

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	output, err := toolset.ReadFile("sample.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(output, "...[truncated]") {
		t.Fatalf("ReadFile() expected truncation marker, got %q", output[len(output)-32:])
	}
}

func TestReadFileRejectsBinaryAndSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	binaryPath := filepath.Join(root, "binary.bin")
	escapedFile := filepath.Join(outside, "secret.txt")
	escapedLink := filepath.Join(root, "secret-link.txt")

	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("WriteFile(binary) error = %v", err)
	}
	mustWriteFile(t, escapedFile, "secret")
	if err := os.Symlink(escapedFile, escapedLink); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	if _, err := toolset.ReadFile("binary.bin"); err == nil {
		t.Fatal("ReadFile(binary) expected error")
	}
	if _, err := toolset.ReadFile("secret-link.txt"); err == nil {
		t.Fatal("ReadFile(symlink escape) expected error")
	}
}

func TestSearchTextFindsMatchesAndRespectsCaseSensitivity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "one.txt"), "Alpha\nbeta\n")
	mustWriteFile(t, filepath.Join(root, "two.txt"), "gamma\nALPHA\n")

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	output, err := toolset.SearchText("alpha", ".", false)
	if err != nil {
		t.Fatalf("SearchText() error = %v", err)
	}
	if !strings.Contains(output, "one.txt:1:Alpha") {
		t.Fatalf("case-insensitive output missing one.txt match: %q", output)
	}
	if !strings.Contains(output, "two.txt:2:ALPHA") {
		t.Fatalf("case-insensitive output missing two.txt match: %q", output)
	}

	output, err = toolset.SearchText("alpha", ".", true)
	if err != nil {
		t.Fatalf("SearchText(case-sensitive) error = %v", err)
	}
	if output != "no matches" {
		t.Fatalf("SearchText(case-sensitive) = %q, want no matches", output)
	}
}

func TestSearchTextSkipsDotGitDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustWriteFile(t, filepath.Join(root, ".git", "config"), "token=alpha")
	mustWriteFile(t, filepath.Join(root, "visible.txt"), "alpha")

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	output, err := toolset.SearchText("alpha", ".", false)
	if err != nil {
		t.Fatalf("SearchText() error = %v", err)
	}
	if strings.Contains(output, ".git/config") {
		t.Fatalf("SearchText() should skip .git, got %q", output)
	}
	if !strings.Contains(output, "visible.txt:1:alpha") {
		t.Fatalf("SearchText() missing non-.git match: %q", output)
	}
}

func TestExecuteRoutesTools(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.txt"), "content")

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	pwd, err := toolset.Execute(toolNamePWD, "{}")
	if err != nil {
		t.Fatalf("Execute(pwd) error = %v", err)
	}
	if pwd != toolset.Root() {
		t.Fatalf("Execute(pwd) = %q, want %q", pwd, toolset.Root())
	}

	readContent, err := toolset.Execute(toolNameReadFile, `{"path":"a.txt"}`)
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if readContent != "content" {
		t.Fatalf("Execute(read_file) = %q, want content", readContent)
	}

	if _, err := toolset.Execute("unknown", "{}"); err == nil {
		t.Fatal("Execute(unknown) expected error")
	}
}

func TestExecutePWDAcceptsFormattedEmptyJSONObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	if _, err := toolset.Execute(toolNamePWD, "{\n}"); err != nil {
		t.Fatalf("Execute(pwd, formatted empty object) error = %v", err)
	}
	if _, err := toolset.Execute(toolNamePWD, "{ }"); err != nil {
		t.Fatalf("Execute(pwd, spaced empty object) error = %v", err)
	}
}

func TestExecutePWDRejectsUnexpectedArguments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	if _, err := toolset.Execute(toolNamePWD, `{"path":"."}`); err == nil {
		t.Fatal("Execute(pwd, non-empty object) expected error")
	}
}

func TestExecuteRejectsTrailingTopLevelTokens(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.txt"), "content")

	toolset, err := NewToolset(root)
	if err != nil {
		t.Fatalf("NewToolset() error = %v", err)
	}

	if _, err := toolset.Execute(toolNameReadFile, `{"path":"a.txt"}{"extra":true}`); err == nil {
		t.Fatal("Execute(read_file, trailing token) expected error")
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}
