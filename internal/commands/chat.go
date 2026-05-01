package commands

import (
	"context"
	"fmt"
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
		Client:        client,
		Config:        command.State,
		IO:            command.Streams,
		Model:         activeModel,
		WorkspaceRoot: ".",
	}
	if cwd, err := os.Getwd(); err == nil {
		session.WorkspaceRoot = cwd
		loaded, err := instructions.Load(cwd)
		if err != nil {
			return err
		}
		session.Config.SystemPrompt = mergeSystemPrompt(command.State.SystemPrompt, loaded.Content)
	}

	if prompt != "" {
		history := []provider.Message{{
			Role:    "user",
			Content: prompt,
		}}
		assistant, _, err := niviruntime.ExecuteAgentTurn(ctx, session, history)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(command.Streams.Out, assistant.Content); err != nil {
			return err
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
