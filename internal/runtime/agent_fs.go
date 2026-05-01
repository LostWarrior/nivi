package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

func toolApplyPatch(root string, args applyPatchArgs) (string, error) {
	if strings.TrimSpace(args.Path) == "" {
		return "", nivierrors.Validation("runtime.tool_apply_patch", "path is required")
	}
	if args.OldText == "" {
		return "", nivierrors.Validation("runtime.tool_apply_patch", "old_text cannot be empty")
	}
	target, rel, err := resolveSandboxPath(root, args.Path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	text := string(content)
	if !strings.Contains(text, args.OldText) {
		return "", nivierrors.Validation("runtime.tool_apply_patch", "old_text not found in target file")
	}

	updated := strings.Replace(text, args.OldText, args.NewText, 1)
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if writeErr := os.WriteFile(target, []byte(updated), info.Mode()); writeErr != nil {
		return "", writeErr
	}

	before := shortPreview(args.OldText)
	after := shortPreview(args.NewText)
	return fmt.Sprintf("applied patch to %s\nold: %s\nnew: %s", rel, before, after), nil
}

func shortPreview(input string) string {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.ReplaceAll(trimmed, "\n", "\\n")
	if len(trimmed) > 120 {
		return trimmed[:120] + "..."
	}
	return trimmed
}

func resolveSandboxPath(root, requested string) (string, string, error) {
	candidate := strings.TrimSpace(requested)
	if candidate == "" {
		candidate = "."
	}

	joined := candidate
	if !filepath.IsAbs(candidate) {
		joined = filepath.Join(root, candidate)
	}

	absPath, err := filepath.Abs(joined)
	if err != nil {
		return "", "", err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}

	rel, err := filepath.Rel(cleanRoot, absPath)
	if err != nil {
		return "", "", err
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", nivierrors.Validation("runtime.resolve_sandbox_path", "path is outside sandbox")
	}

	return absPath, filepath.ToSlash(rel), nil
}
