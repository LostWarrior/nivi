package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LostWarrior/nivi/internal/config"
	"github.com/LostWarrior/nivi/internal/instructions"
	"github.com/LostWarrior/nivi/internal/provider"
	niviruntime "github.com/LostWarrior/nivi/internal/runtime"
	"github.com/LostWarrior/nivi/internal/selection"
)

type ChatCommand struct {
	State      config.State
	PromptArgs []string
	Streams    niviruntime.IO
}

func RunChat(ctx context.Context, command ChatCommand) error {
	if err := command.State.Validate(); err != nil {
		return err
	}

	client, err := provider.New(command.State)
	if err != nil {
		return err
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		return err
	}

	activeModel, err := selection.Resolve(command.State.DefaultModel, models)
	if err != nil {
		return err
	}

	prompt, err := niviruntime.ReadPrompt(command.Streams, command.PromptArgs)
	if err != nil {
		return err
	}

	session := niviruntime.Session{
		Client: client,
		Config: command.State,
		IO:     command.Streams,
		Model:  activeModel,
	}
	if cwd, err := os.Getwd(); err == nil {
		loaded, err := instructions.Load(cwd)
		if err != nil {
			return err
		}
		session.Config.SystemPrompt = mergeSystemPrompt(command.State.SystemPrompt, loaded.Content)
	}

	if prompt != "" {
		response, err := runChat(
			ctx,
			session,
			buildMessages(session.Config.SystemPrompt, []provider.Message{{
				Role:    "user",
				Content: prompt,
			}}),
		)
		if err != nil {
			return err
		}

		if !shouldStream(command.State, command.Streams) {
			if _, err := fmt.Fprintln(command.Streams.Out, response); err != nil {
				return err
			}
		}

		return nil
	}

	return niviruntime.RunREPL(ctx, session)
}

func mergeSystemPrompt(basePrompt string, extraPrompt string) string {
	basePrompt = strings.TrimSpace(basePrompt)
	extraPrompt = strings.TrimSpace(extraPrompt)

	switch {
	case basePrompt == "":
		return extraPrompt
	case extraPrompt == "":
		return basePrompt
	default:
		return basePrompt + "\n\n" + extraPrompt
	}
}

func buildMessages(systemPrompt string, history []provider.Message) []provider.Message {
	messages := make([]provider.Message, 0, len(history)+1)
	messages = append(messages, provider.Message{
		Role:    "system",
		Content: systemPrompt,
	})
	messages = append(messages, history...)
	return messages
}

func runChat(ctx context.Context, session niviruntime.Session, messages []provider.Message) (string, error) {
	request := provider.ChatRequest{
		Model:     session.Model,
		Messages:  messages,
		MaxTokens: session.Config.MaxTokens,
		Stream:    shouldStream(session.Config, session.IO),
	}

	if request.Stream {
		response, err := session.Client.Stream(ctx, request, func(delta string) error {
			_, writeErr := io.WriteString(session.IO.Out, delta)
			return writeErr
		})
		if err != nil {
			return "", err
		}

		if _, err := fmt.Fprintln(session.IO.Out); err != nil {
			return "", err
		}

		return response, nil
	}

	return session.Client.Complete(ctx, request)
}

func shouldStream(state config.State, streams niviruntime.IO) bool {
	return state.StreamingEnabled && streams.StdoutTTY
}
