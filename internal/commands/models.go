package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/LostWarrior/nivi/internal/config"
	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
	"github.com/LostWarrior/nivi/internal/selection"
)

type ModelsResult struct {
	Active string   `json:"active_model"`
	Models []string `json:"models"`
}

func RunModels(ctx context.Context, stdout io.Writer, client *provider.Client, state config.State, jsonOutput bool) error {
	if stdout == nil {
		stdout = os.Stdout
	}
	if client == nil {
		var err error
		client, err = provider.New(state)
		if err != nil {
			return err
		}
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		return err
	}

	activeModel, err := selection.Resolve(state.DefaultModel, models)
	if err != nil {
		return err
	}

	grouped := selection.Group(models, activeModel)
	orderedModels := make([]string, 0, 1+len(grouped.Recommended)+len(grouped.Others))
	orderedModels = append(orderedModels, activeModel)
	orderedModels = append(orderedModels, grouped.Recommended...)
	orderedModels = append(orderedModels, grouped.Others...)

	if jsonOutput {
		return json.NewEncoder(stdout).Encode(ModelsResult{
			Active: activeModel,
			Models: orderedModels,
		})
	}

	if _, err := fmt.Fprintf(stdout, "* %s\n", activeModel); err != nil {
		return nivierrors.Network("commands.models", err)
	}

	if len(grouped.Recommended) > 0 {
		if _, err := fmt.Fprintln(stdout, "\nRecommended:"); err != nil {
			return nivierrors.Network("commands.models", err)
		}
		for _, modelID := range grouped.Recommended {
			if _, err := fmt.Fprintf(stdout, "%s\n", modelID); err != nil {
				return nivierrors.Network("commands.models", err)
			}
		}
	}

	if len(grouped.Others) > 0 {
		if _, err := fmt.Fprintln(stdout, "\nAvailable:"); err != nil {
			return nivierrors.Network("commands.models", err)
		}
		for _, modelID := range grouped.Others {
			if _, err := fmt.Fprintf(stdout, "%s\n", modelID); err != nil {
				return nivierrors.Network("commands.models", err)
			}
		}
	}

	return nil
}
