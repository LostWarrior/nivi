package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LostWarrior/nivi/internal/config"
	"github.com/LostWarrior/nivi/internal/provider"
	"github.com/LostWarrior/nivi/internal/selection"
)

type Session struct {
	Client *provider.Client
	Config config.State
	IO     IO
	Model  string
}

func RunREPL(ctx context.Context, session Session) error {
	reader := bufio.NewReader(session.IO.In)
	history := make([]provider.Message, 0, 16)

	if _, err := fmt.Fprintf(session.IO.Err, "nivi using %s\n", session.Model); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(session.IO.Err, "Commands: /exit, /quit, /clear, /model"); err != nil {
		return err
	}

	for {
		if _, err := fmt.Fprint(session.IO.Err, "\n>>> "); err != nil {
			return err
		}

		prompt, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				if _, printErr := fmt.Fprintln(session.IO.Err); printErr != nil {
					return printErr
				}
				return nil
			}

			return err
		}

		switch {
		case prompt == "":
			continue
		case prompt == "/exit" || prompt == "/quit":
			return nil
		case prompt == "/clear":
			history = history[:0]
			if _, err := fmt.Fprintln(session.IO.Err, "Conversation cleared."); err != nil {
				return err
			}
			continue
		case isModelCommand(prompt):
			nextModel, handled, err := handleModelCommand(ctx, reader, session, prompt)
			if err != nil {
				if _, writeErr := fmt.Fprintf(session.IO.Err, "error: %v\n", err); writeErr != nil {
					return writeErr
				}
				continue
			}
			if handled {
				session.Model = nextModel
			}
			continue
		}

		history = append(history, provider.Message{
			Role:    "user",
			Content: prompt,
		})

		assistant, nextHistory, err := ExecuteAgentTurn(ctx, session, history)
		if err != nil {
			history = history[:len(history)-1]
			if _, writeErr := fmt.Fprintf(session.IO.Err, "error: %v\n", err); writeErr != nil {
				return writeErr
			}
			continue
		}
		history = nextHistory
		if _, err := fmt.Fprintln(session.IO.Out, assistant.Content); err != nil {
			return err
		}
	}
}

func handleModelCommand(ctx context.Context, reader *bufio.Reader, session Session, input string) (string, bool, error) {
	modelArg := strings.TrimSpace(strings.TrimPrefix(input, "/model"))
	models, err := session.Client.ListModels(ctx)
	if err != nil {
		return session.Model, false, err
	}

	if modelArg != "" {
		model, err := selection.Resolve(modelArg, models)
		if err != nil {
			return session.Model, false, err
		}

		if _, err := fmt.Fprintf(session.IO.Err, "Switched to %s\n", model); err != nil {
			return session.Model, false, err
		}
		return model, true, nil
	}

	if !session.IO.StdinTTY {
		if _, err := fmt.Fprintln(session.IO.Err, session.Model); err != nil {
			return session.Model, false, err
		}
		return session.Model, false, nil
	}

	grouped := selection.Group(models, session.Model)
	ordered := make([]string, 0, 1+len(grouped.Recommended)+len(grouped.Others))
	ordered = append(ordered, session.Model)
	ordered = append(ordered, grouped.Recommended...)
	ordered = append(ordered, grouped.Others...)

	if _, err := fmt.Fprintf(session.IO.Err, "Active model: %s\n\n", session.Model); err != nil {
		return session.Model, false, err
	}

	if _, err := fmt.Fprintln(session.IO.Err, "Recommended"); err != nil {
		return session.Model, false, err
	}
	if _, err := fmt.Fprintf(session.IO.Err, "[1] %s (current)\n", session.Model); err != nil {
		return session.Model, false, err
	}
	for index, modelID := range grouped.Recommended {
		if _, err := fmt.Fprintf(session.IO.Err, "[%d] %s\n", index+2, modelID); err != nil {
			return session.Model, false, err
		}
	}

	if len(grouped.Others) > 0 {
		if _, err := fmt.Fprintln(session.IO.Err, "\nAvailable"); err != nil {
			return session.Model, false, err
		}
		for index, modelID := range grouped.Others {
			if _, err := fmt.Fprintf(session.IO.Err, "[%d] %s\n", index+len(grouped.Recommended)+2, modelID); err != nil {
				return session.Model, false, err
			}
		}
	}

	if _, err := fmt.Fprint(session.IO.Err, "\nSelect a number or paste a model id: "); err != nil {
		return session.Model, false, err
	}

	selectionInput, err := readLine(reader)
	if err != nil {
		if err == io.EOF {
			return session.Model, false, nil
		}
		return session.Model, false, err
	}

	if selectionInput == "" {
		if _, err := fmt.Fprintln(session.IO.Err, "Model unchanged."); err != nil {
			return session.Model, false, err
		}
		return session.Model, false, nil
	}

	if index, err := strconv.Atoi(selectionInput); err == nil {
		if index < 1 || index > len(ordered) {
			return session.Model, false, fmt.Errorf("invalid model selection: %d", index)
		}
		nextModel := ordered[index-1]
		if _, err := fmt.Fprintf(session.IO.Err, "Switched to %s\n", nextModel); err != nil {
			return session.Model, false, err
		}
		return nextModel, true, nil
	}

	nextModel, err := selection.Resolve(selectionInput, models)
	if err != nil {
		return session.Model, false, err
	}

	if _, err := fmt.Fprintf(session.IO.Err, "Switched to %s\n", nextModel); err != nil {
		return session.Model, false, err
	}

	return nextModel, true, nil
}

func isModelCommand(input string) bool {
	if input == "/model" {
		return true
	}
	return strings.HasPrefix(input, "/model ") || strings.HasPrefix(input, "/model\t")
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(line), err
}
