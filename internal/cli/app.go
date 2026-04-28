package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/LostWarrior/nivi/internal/commands"
	"github.com/LostWarrior/nivi/internal/config"
	"github.com/LostWarrior/nivi/internal/provider"
	niviruntime "github.com/LostWarrior/nivi/internal/runtime"
)

const version = "dev"

type rootCommand struct {
	Name    string
	Prompt  []string
	Options config.Options
	JSON    bool
	ShowHelp bool
}

func Run(args []string, streams niviruntime.IO) int {
	command, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
		return 1
	}

	if command.ShowHelp {
		printHelp(streams.Out)
		return 0
	}

	if command.Name == "version" {
		_, _ = fmt.Fprintln(streams.Out, version)
		return 0
	}

	state := config.Resolve(command.Options)
	ctx := context.Background()

	switch command.Name {
	case "models":
		if err := state.Validate(); err != nil {
			_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
			return 1
		}

		client, err := provider.New(state)
		if err != nil {
			_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
			return 1
		}

		if err := commands.RunModels(ctx, streams.Out, client, state, command.JSON); err != nil {
			_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
			return 1
		}

		return 0
	case "doctor":
		if err := commands.RunDoctor(ctx, streams.Out, state, command.JSON); err != nil {
			_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
			return 1
		}

		return 0
	default:
		if err := commands.RunChat(ctx, commands.ChatCommand{
			State:      state,
			PromptArgs: command.Prompt,
			Streams:    streams,
		}); err != nil {
			_, _ = fmt.Fprintln(streams.Err, strings.TrimSpace(err.Error()))
			return 1
		}

		return 0
	}
}

func parseArgs(args []string) (rootCommand, error) {
	if len(args) == 0 {
		return parseChatCommand(nil)
	}

	switch args[0] {
	case "help", "--help", "-h":
		return rootCommand{Name: "help", ShowHelp: true}, nil
	case "models":
		return parseModelsCommand(args[1:])
	case "doctor":
		return parseDoctorCommand(args[1:])
	case "chat":
		return parseChatCommand(args[1:])
	case "version", "--version", "-v":
		return rootCommand{Name: "version"}, nil
	default:
		return parseChatCommand(args)
	}
}

func parseChatCommand(args []string) (rootCommand, error) {
	flagSet := flag.NewFlagSet("nivi", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var options config.Options
	flagSet.StringVar(&options.Model, "m", "", "model id")
	flagSet.StringVar(&options.Model, "model", "", "model id")
	flagSet.StringVar(&options.BaseURL, "base-url", "", "base API URL")
	flagSet.StringVar(&options.SystemPrompt, "system", "", "system prompt")
	flagSet.IntVar(&options.MaxTokens, "max-tokens", config.DefaultMaxTokens, "max tokens")
	flagSet.BoolVar(&options.DisableStreaming, "no-stream", false, "disable streaming")
	if err := flagSet.Parse(args); err != nil {
		return rootCommand{}, err
	}

	return rootCommand{
		Name:    "chat",
		Prompt:  flagSet.Args(),
		Options: options,
	}, nil
}

func parseModelsCommand(args []string) (rootCommand, error) {
	flagSet := flag.NewFlagSet("nivi models", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var options config.Options
	var jsonOutput bool
	flagSet.StringVar(&options.Model, "m", "", "model id")
	flagSet.StringVar(&options.Model, "model", "", "model id")
	flagSet.StringVar(&options.BaseURL, "base-url", "", "base API URL")
	flagSet.BoolVar(&jsonOutput, "json", false, "JSON output")
	if err := flagSet.Parse(args); err != nil {
		return rootCommand{}, err
	}

	return rootCommand{
		Name:    "models",
		Options: options,
		JSON:    jsonOutput,
	}, nil
}

func parseDoctorCommand(args []string) (rootCommand, error) {
	flagSet := flag.NewFlagSet("nivi doctor", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var options config.Options
	var jsonOutput bool
	flagSet.StringVar(&options.Model, "m", "", "model id")
	flagSet.StringVar(&options.Model, "model", "", "model id")
	flagSet.StringVar(&options.BaseURL, "base-url", "", "base API URL")
	flagSet.BoolVar(&jsonOutput, "json", false, "JSON output")
	if err := flagSet.Parse(args); err != nil {
		return rootCommand{}, err
	}

	return rootCommand{
		Name:    "doctor",
		Options: options,
		JSON:    jsonOutput,
	}, nil
}

func printHelp(output io.Writer) {
	_, _ = fmt.Fprintln(output, "Usage:")
	_, _ = fmt.Fprintln(output, "  nivi [flags] [prompt]")
	_, _ = fmt.Fprintln(output, "  nivi chat [flags] [prompt]")
	_, _ = fmt.Fprintln(output, "  nivi models [--json] [-m model]")
	_, _ = fmt.Fprintln(output, "  nivi doctor [--json] [-m model]")
	_, _ = fmt.Fprintln(output, "  nivi version")
	_, _ = fmt.Fprintln(output)
	_, _ = fmt.Fprintln(output, "Interactive commands:")
	_, _ = fmt.Fprintln(output, "  /clear")
	_, _ = fmt.Fprintln(output, "  /model")
	_, _ = fmt.Fprintln(output, "  /model <id>")
	_, _ = fmt.Fprintln(output, "  /quit")
	_, _ = fmt.Fprintln(output, "  /exit")
}
