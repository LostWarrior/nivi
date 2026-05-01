package instructions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxInstructionBytes = 1 << 20

var candidateNames = []string{
	"NIVI.md",
	"nivi.md",
	"AGENT.md",
	"agent.md",
	"CLAUDE.md",
	"claude.md",
	".nivi/NIVI.md",
	".nivi/nivi.md",
	".nivi/AGENT.md",
	".nivi/agent.md",
	".nivi/CLAUDE.md",
	".nivi/claude.md",
}

type Loaded struct {
	Paths   []string
	Content string
}

func Load(startDir string) (Loaded, error) {
	if strings.TrimSpace(startDir) == "" {
		return Loaded{}, nil
	}

	absoluteStart, err := filepath.Abs(startDir)
	if err != nil {
		return Loaded{}, err
	}

	directories := ancestorDirs(absoluteStart)
	paths := make([]string, 0, len(directories))
	sections := make([]string, 0, len(directories))
	totalBytes := 0

	for _, dir := range directories {
		matchedPaths, err := existingPaths(dir)
		if err != nil {
			return Loaded{}, err
		}
		for _, path := range matchedPaths {
			content, err := readFile(path, maxInstructionBytes-totalBytes)
			if err != nil {
				return Loaded{}, err
			}
			if strings.TrimSpace(content) == "" {
				continue
			}

			paths = append(paths, path)
			sections = append(sections, fmt.Sprintf("File: %s\n%s", path, content))
			totalBytes += len(content)
			if totalBytes >= maxInstructionBytes {
				break
			}
		}
		if totalBytes >= maxInstructionBytes {
			break
		}
	}

	if len(sections) == 0 {
		return Loaded{}, nil
	}

	return Loaded{
		Paths: paths,
		Content: strings.Join([]string{
			"Follow these project instructions in addition to the base system prompt.",
			strings.Join(sections, "\n\n"),
		}, "\n\n"),
	}, nil
}

func ancestorDirs(start string) []string {
	current := filepath.Clean(start)
	dirs := []string{current}

	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		dirs = append(dirs, parent)
		current = parent
	}

	for left, right := 0, len(dirs)-1; left < right; left, right = left+1, right-1 {
		dirs[left], dirs[right] = dirs[right], dirs[left]
	}

	return dirs
}

func existingPaths(dir string) ([]string, error) {
	paths := make([]string, 0, len(candidateNames))
	seenInfos := make([]os.FileInfo, 0, len(candidateNames))

	for _, name := range candidateNames {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			isDuplicate := false
			for _, seen := range seenInfos {
				if os.SameFile(seen, info) {
					isDuplicate = true
					break
				}
			}
			if isDuplicate {
				continue
			}
			paths = append(paths, path)
			seenInfos = append(seenInfos, info)
		case err == nil && info.IsDir():
			continue
		case os.IsNotExist(err):
			continue
		case err != nil:
			return nil, err
		}
	}

	return paths, nil
}

func readFile(path string, remaining int) (string, error) {
	if remaining <= 0 {
		return "", nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, int64(remaining)))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}
