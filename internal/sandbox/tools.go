package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
)

const (
	defaultMaxReadBytes    = 64 << 10
	defaultMaxOutputBytes  = 64 << 10
	defaultMaxDirEntries   = 512
	defaultMaxSearchHits   = 200
	defaultMaxLineChars    = 300
	toolTypeFunction       = "function"
	toolNamePWD            = "pwd"
	toolNameLS             = "ls"
	toolNameReadFile       = "read_file"
	toolNameSearchText     = "search_text"
	opSandboxNew           = "sandbox.new"
	opSandboxExecute       = "sandbox.execute"
	opSandboxDecodeArgs    = "sandbox.decode_args"
	opSandboxUnknownTool   = "sandbox.unknown_tool"
	opSandboxInvalidRoot   = "sandbox.invalid_root"
	opSandboxInvalidArg    = "sandbox.invalid_arg"
	opSandboxPathTraversal = "sandbox.path_traversal"
)

var errToolHasNoArguments = errors.New("tool does not accept arguments")

type Toolset struct {
	root           string
	maxReadBytes   int
	maxOutputBytes int
	maxDirEntries  int
	maxSearchHits  int
	maxLineChars   int
}

func NewToolset(root string) (*Toolset, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nivierrors.Validation(opSandboxInvalidRoot, "sandbox root cannot be empty")
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nivierrors.Validation(opSandboxInvalidRoot, "failed to resolve sandbox root")
	}

	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, nivierrors.Validation(opSandboxInvalidRoot, "sandbox root is not accessible")
	}

	return &Toolset{
		root:           resolvedRoot,
		maxReadBytes:   defaultMaxReadBytes,
		maxOutputBytes: defaultMaxOutputBytes,
		maxDirEntries:  defaultMaxDirEntries,
		maxSearchHits:  defaultMaxSearchHits,
		maxLineChars:   defaultMaxLineChars,
	}, nil
}

func (t *Toolset) Root() string {
	return t.root
}

func (t *Toolset) Definitions() []provider.Tool {
	return ReadOnlyDefinitions()
}

func ReadOnlyDefinitions() []provider.Tool {
	return []provider.Tool{
		{
			Type: toolTypeFunction,
			Function: provider.ToolFunction{
				Name:        toolNamePWD,
				Description: "Print the current sandbox root directory.",
				Parameters:  rawSchema(`{"type":"object","additionalProperties":false}`),
			},
		},
		{
			Type: toolTypeFunction,
			Function: provider.ToolFunction{
				Name:        toolNameLS,
				Description: "List files and directories under a path within the sandbox.",
				Parameters: rawSchema(`{
					"type":"object",
					"additionalProperties":false,
					"properties":{"path":{"type":"string","description":"Optional path inside sandbox. Defaults to current root."}}
				}`),
			},
		},
		{
			Type: toolTypeFunction,
			Function: provider.ToolFunction{
				Name:        toolNameReadFile,
				Description: "Read a UTF-8 text file within the sandbox.",
				Parameters: rawSchema(`{
					"type":"object",
					"additionalProperties":false,
					"required":["path"],
					"properties":{"path":{"type":"string","description":"File path inside sandbox."}}
				}`),
			},
		},
		{
			Type: toolTypeFunction,
			Function: provider.ToolFunction{
				Name:        toolNameSearchText,
				Description: "Search for text in files under a sandbox path.",
				Parameters: rawSchema(`{
					"type":"object",
					"additionalProperties":false,
					"required":["query"],
					"properties":{
						"query":{"type":"string","description":"Text query to search for."},
						"path":{"type":"string","description":"Optional root path inside sandbox."},
						"case_sensitive":{"type":"boolean","description":"Set true to match case exactly."}
					}
				}`),
			},
		},
	}
}

func (t *Toolset) Execute(name string, arguments string) (string, error) {
	name = strings.TrimSpace(name)
	switch name {
	case toolNamePWD:
		var args struct{}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return "", nivierrors.Validation(opSandboxDecodeArgs, errToolHasNoArguments.Error())
		}
		return t.PWD(), nil
	case toolNameLS:
		var args lsArgs
		if err := decodeToolArgs(arguments, &args); err != nil {
			return "", err
		}
		return t.LS(args.Path)
	case toolNameReadFile:
		var args readFileArgs
		if err := decodeToolArgs(arguments, &args); err != nil {
			return "", err
		}
		return t.ReadFile(args.Path)
	case toolNameSearchText:
		var args searchTextArgs
		if err := decodeToolArgs(arguments, &args); err != nil {
			return "", err
		}
		return t.SearchText(args.Query, args.Path, args.CaseSensitive)
	default:
		return "", nivierrors.Validation(opSandboxUnknownTool, fmt.Sprintf("unsupported tool %q", name))
	}
}

type lsArgs struct {
	Path string `json:"path"`
}

type readFileArgs struct {
	Path string `json:"path"`
}

type searchTextArgs struct {
	Query         string `json:"query"`
	Path          string `json:"path"`
	CaseSensitive bool   `json:"case_sensitive"`
}

func decodeToolArgs(input string, out any) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		trimmed = "{}"
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return nivierrors.Validation(opSandboxInvalidArg, "tool arguments must be valid JSON object")
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nivierrors.Validation(opSandboxInvalidArg, "tool arguments must be a single JSON object")
	}
	return nil
}

func rawSchema(value string) json.RawMessage {
	return json.RawMessage(value)
}
