package sandbox

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

func (t *Toolset) PWD() string {
	return t.root
}

func (t *Toolset) LS(inputPath string) (string, error) {
	targetPath, err := t.resolvePath(inputPath, true)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "path is not accessible")
	}
	if !info.IsDir() {
		return "", nivierrors.Validation(opSandboxExecute, "ls requires a directory path")
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "failed to list directory")
	}

	lines := make([]string, 0, len(entries)+1)
	for index, entry := range entries {
		if index >= t.maxDirEntries {
			lines = append(lines, "... (truncated)")
			break
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	if len(lines) == 0 {
		return "(empty)", nil
	}
	return trimOutput(strings.Join(lines, "\n"), t.maxOutputBytes), nil
}

func (t *Toolset) ReadFile(inputPath string) (string, error) {
	candidatePath := strings.TrimSpace(inputPath)
	if candidatePath == "" {
		return "", nivierrors.Validation(opSandboxInvalidArg, "read_file requires a path")
	}

	targetPath, err := t.resolvePath(candidatePath, true)
	if err != nil {
		return "", err
	}
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "file is not accessible")
	}
	if fileInfo.IsDir() {
		return "", nivierrors.Validation(opSandboxExecute, "read_file requires a file path")
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "file cannot be opened")
	}
	defer file.Close()

	limitReader := io.LimitReader(file, int64(t.maxReadBytes+1))
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "file read failed")
	}

	truncated := len(content) > t.maxReadBytes
	if truncated {
		content = content[:t.maxReadBytes]
	}
	if !isLikelyUTF8Text(content) {
		return "", nivierrors.Validation(opSandboxExecute, "read_file only supports UTF-8 text files")
	}

	output := string(content)
	if truncated {
		output += "\n\n...[truncated]"
	}
	return trimOutput(output, t.maxOutputBytes), nil
}

func (t *Toolset) SearchText(query, inputPath string, caseSensitive bool) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nivierrors.Validation(opSandboxInvalidArg, "search_text requires a non-empty query")
	}

	basePath, err := t.resolvePath(inputPath, true)
	if err != nil {
		return "", err
	}
	baseInfo, err := os.Stat(basePath)
	if err != nil {
		return "", nivierrors.Validation(opSandboxExecute, "path is not accessible")
	}

	searchNeedle := query
	if !caseSensitive {
		searchNeedle = strings.ToLower(query)
	}

	lines := make([]string, 0, 32)
	matchCount := 0
	walkErr := filepath.WalkDir(basePath, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil
		}
		if !t.pathStaysInRoot(resolvedPath) {
			return nil
		}
		if !isLikelyTextFile(resolvedPath) {
			return nil
		}

		file, err := os.Open(resolvedPath)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			content := scanner.Text()
			candidate := content
			if !caseSensitive {
				candidate = strings.ToLower(candidate)
			}
			if !strings.Contains(candidate, searchNeedle) {
				continue
			}

			matchCount++
			relativePath := toRelativePath(t.root, path)
			lines = append(lines, relativePath+":"+strconv.Itoa(lineNumber)+":"+truncateLine(content, t.maxLineChars))
			if matchCount >= t.maxSearchHits {
				lines = append(lines, "... (truncated)")
				return fs.SkipAll
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != fs.SkipAll {
		return "", nivierrors.Validation(opSandboxExecute, "search failed")
	}
	if !baseInfo.IsDir() && len(lines) == 0 {
		return "no matches", nil
	}
	if len(lines) == 0 {
		return "no matches", nil
	}

	return trimOutput(strings.Join(lines, "\n"), t.maxOutputBytes), nil
}

func (t *Toolset) resolvePath(input string, mustExist bool) (string, error) {
	candidate := strings.TrimSpace(input)
	if candidate == "" {
		candidate = "."
	}

	target := candidate
	if !filepath.IsAbs(target) {
		target = filepath.Join(t.root, target)
	}

	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return "", nivierrors.Validation(opSandboxPathTraversal, "path cannot be resolved")
	}
	if mustExist {
		absoluteTarget, err = filepath.EvalSymlinks(absoluteTarget)
		if err != nil {
			return "", nivierrors.Validation(opSandboxExecute, "path is not accessible")
		}
	}
	if !t.pathStaysInRoot(absoluteTarget) {
		return "", nivierrors.Validation(opSandboxPathTraversal, "path escapes sandbox root")
	}
	return absoluteTarget, nil
}

func (t *Toolset) pathStaysInRoot(candidate string) bool {
	relativePath, err := filepath.Rel(t.root, candidate)
	if err != nil {
		return false
	}
	if relativePath == ".." {
		return false
	}
	return !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func isLikelyTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buffer := make([]byte, 4096)
	count, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}
	return isLikelyUTF8Text(buffer[:count])
}

func isLikelyUTF8Text(content []byte) bool {
	for _, value := range content {
		if value == 0 {
			return false
		}
	}
	return utf8.Valid(content)
}

func trimOutput(input string, maxBytes int) string {
	if maxBytes <= 0 || len(input) <= maxBytes {
		return input
	}
	const suffix = "\n...[truncated]"
	keep := maxBytes - len(suffix)
	if keep <= 0 {
		return suffix
	}
	return input[:keep] + suffix
}

func truncateLine(input string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= maxChars {
		return input
	}
	return string(runes[:maxChars]) + "..."
}

func toRelativePath(root, path string) string {
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(relativePath)
}
