package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
)

const (
	maxToolOutputBytes = 64 << 10
	maxSearchHits      = 200
	maxReadBytes       = 128 << 10
)

func agentTools() []provider.Tool {
	return []provider.Tool{
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "pwd",
				Description: "Return the sandbox root directory for this chat session.",
			},
		},
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "ls",
				Description: "List files and directories under a path within the sandbox.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"additionalProperties":false}`),
			},
		},
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "read_file",
				Description: "Read a UTF-8 text file from the sandbox. Optionally select line range.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"start_line":{"type":"integer","minimum":1},"end_line":{"type":"integer","minimum":1}},"required":["path"],"additionalProperties":false}`),
			},
		},
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "search_text",
				Description: "Search plain text files in sandbox for a query string.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string"}},"required":["query"],"additionalProperties":false}`),
			},
		},
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "apply_patch",
				Description: "Apply a single exact-text replacement to a sandbox file. Requires user approval.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"old_text":{"type":"string"},"new_text":{"type":"string"}},"required":["path","old_text","new_text"],"additionalProperties":false}`),
			},
		},
	}
}

type lsArgs struct {
	Path string `json:"path"`
}

type readFileArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type searchTextArgs struct {
	Query string `json:"query"`
	Path  string `json:"path"`
}

type applyPatchArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func executeToolCall(session Session, root string, call provider.ToolCall) provider.Message {
	content, err := dispatchTool(session, root, call.Function.Name, call.Function.Arguments)
	if err != nil {
		content = "error: " + nivierrors.SafeMessage(err)
	}
	if len(content) > maxToolOutputBytes {
		content = content[:maxToolOutputBytes] + "\n... [truncated]"
	}
	content = nivierrors.RedactSecrets(content)

	return provider.Message{
		Role:       "tool",
		Name:       call.Function.Name,
		ToolCallID: call.ID,
		Content:    content,
	}
}

func dispatchTool(session Session, root, name, rawArgs string) (string, error) {
	switch name {
	case "pwd":
		return root, nil
	case "ls":
		var args lsArgs
		if err := decodeToolArgs(rawArgs, &args); err != nil {
			return "", err
		}
		return toolLS(root, args.Path)
	case "read_file":
		var args readFileArgs
		if err := decodeToolArgs(rawArgs, &args); err != nil {
			return "", err
		}
		return toolReadFile(root, args)
	case "search_text":
		var args searchTextArgs
		if err := decodeToolArgs(rawArgs, &args); err != nil {
			return "", err
		}
		return toolSearchText(root, args)
	case "apply_patch":
		var args applyPatchArgs
		if err := decodeToolArgs(rawArgs, &args); err != nil {
			return "", err
		}
		if err := confirmApplyPatch(session, args); err != nil {
			return "", err
		}
		return toolApplyPatch(root, args)
	default:
		return "", nivierrors.Validation("runtime.dispatch_tool", fmt.Sprintf("unknown tool: %s", name))
	}
}

func decodeToolArgs(raw string, out any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "{}"
	}
	if err := json.Unmarshal([]byte(trimmed), out); err != nil {
		return nivierrors.Validation("runtime.decode_tool_args", "tool arguments must be valid JSON")
	}
	return nil
}

func confirmApplyPatch(session Session, args applyPatchArgs) error {
	if !session.IO.StdinTTY {
		return nivierrors.Validation("runtime.confirm_apply_patch", "apply_patch requires interactive approval")
	}
	_, _ = fmt.Fprintln(session.IO.Err, "\nTool approval required: apply_patch")
	_, _ = fmt.Fprintf(session.IO.Err, "Path: %s\n", args.Path)
	_, _ = fmt.Fprintln(session.IO.Err, "Proceed? [y/N]: ")

	reader := bufio.NewReader(session.IO.In)
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	choice := strings.ToLower(strings.TrimSpace(input))
	if choice != "y" && choice != "yes" {
		return nivierrors.Validation("runtime.confirm_apply_patch", "apply_patch declined by user")
	}
	return nil
}

func toolLS(root, requested string) (string, error) {
	target, rel, err := resolveSandboxPath(root, requested)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var lines []string
	lines = append(lines, "path: "+rel)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return strings.Join(lines, "\n"), nil
}

func toolReadFile(root string, args readFileArgs) (string, error) {
	if strings.TrimSpace(args.Path) == "" {
		return "", nivierrors.Validation("runtime.tool_read_file", "path is required")
	}
	target, rel, err := resolveSandboxPath(root, args.Path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	if len(content) > maxReadBytes {
		content = content[:maxReadBytes]
	}
	lines := strings.Split(string(content), "\n")
	start := 1
	if args.StartLine > 0 {
		start = args.StartLine
	}
	end := len(lines)
	if args.EndLine > 0 && args.EndLine < end {
		end = args.EndLine
	}
	if start > end || start > len(lines) {
		return fmt.Sprintf("path: %s\n(empty range)", rel), nil
	}
	picked := lines[start-1 : end]
	return fmt.Sprintf("path: %s\n%s", rel, strings.Join(picked, "\n")), nil
}

func toolSearchText(root string, args searchTextArgs) (string, error) {
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", nivierrors.Validation("runtime.tool_search_text", "query is required")
	}
	target, _, err := resolveSandboxPath(root, args.Path)
	if err != nil {
		return "", err
	}

	hits := make([]string, 0, 32)
	err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if len(data) > maxReadBytes {
			data = data[:maxReadBytes]
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relPath = path
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, query) {
				hits = append(hits, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(relPath), i+1, line))
				if len(hits) >= maxSearchHits {
					return io.EOF
				}
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	if len(hits) == 0 {
		return "no matches", nil
	}
	return strings.Join(hits, "\n"), nil
}
