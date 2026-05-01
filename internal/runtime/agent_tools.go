package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
	"github.com/LostWarrior/nivi/internal/sandbox"
)

const maxToolOutputBytes = 64 << 10

type applyPatchArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func agentTools() []provider.Tool {
	tools := sandbox.ReadOnlyDefinitions()
	tools = append(tools, provider.Tool{
		Type: "function",
		Function: provider.ToolFunction{
			Name:        "apply_patch",
			Description: "Apply a single exact-text replacement to a sandbox file. Requires user approval.",
			Parameters: json.RawMessage(`{
				"type":"object",
				"additionalProperties":false,
				"required":["path","old_text","new_text"],
				"properties":{
					"path":{"type":"string"},
					"old_text":{"type":"string"},
					"new_text":{"type":"string"}
				}
			}`),
		},
	})
	return tools
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
	if name == "apply_patch" {
		var args applyPatchArgs
		if err := decodeToolArgs(rawArgs, &args); err != nil {
			return "", err
		}
		if err := confirmApplyPatch(session, args); err != nil {
			return "", err
		}
		return toolApplyPatch(root, args)
	}

	toolset, err := sandbox.NewToolset(root)
	if err != nil {
		return "", err
	}
	return toolset.Execute(name, rawArgs)
}

func decodeToolArgs(raw string, out any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "{}"
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return nivierrors.Validation("runtime.decode_tool_args", "tool arguments must be valid JSON object")
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nivierrors.Validation("runtime.decode_tool_args", "tool arguments must be a single JSON object")
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
