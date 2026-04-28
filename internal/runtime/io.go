package runtime

import (
	"fmt"
	"io"
	"os"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

const maxPromptBytes = 1 << 20

type IO struct {
	In        io.Reader
	Out       io.Writer
	Err       io.Writer
	StdinTTY  bool
	StdoutTTY bool
}

func IsTTY(file *os.File) bool {
	fileInfo, err := file.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func ReadPrompt(streams IO, promptArgs []string) (string, error) {
	var stdinContent string
	if !streams.StdinTTY {
		content, err := io.ReadAll(io.LimitReader(streams.In, maxPromptBytes+1))
		if err != nil {
			return "", nivierrors.Wrap(nivierrors.KindNetwork, "failed to read stdin", err)
		}

		if len(content) > maxPromptBytes {
			return "", nivierrors.New(nivierrors.KindValidation, "stdin input is too large; keep input below 1 MiB")
		}

		stdinContent = strings.TrimSpace(string(content))
	}

	argumentPrompt := strings.TrimSpace(strings.Join(promptArgs, " "))
	switch {
	case argumentPrompt != "" && stdinContent != "":
		return fmt.Sprintf("%s\n\n%s", argumentPrompt, stdinContent), nil
	case argumentPrompt != "":
		return argumentPrompt, nil
	default:
		return stdinContent, nil
	}
}
