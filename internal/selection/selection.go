package selection

import (
	"sort"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
)

var recommendedOrder = []string{
	"meta/llama-3.3-70b-instruct",
	"meta/llama-3.1-405b-instruct",
	"meta/llama-3.1-70b-instruct",
	"nvidia/llama-3.1-nemotron-70b-instruct",
	"mistralai/mixtral-8x7b-instruct-v0.1",
}

type GroupedModels struct {
	Active      string
	Recommended []string
	Others      []string
}

func Resolve(requested string, models []provider.Model) (string, error) {
	available := uniqueSortedIDs(models)
	if len(available) == 0 {
		return "", nivierrors.Unavailable(
			"selection.resolve",
			"no models are available to this API key.",
			nil,
		)
	}

	requested = strings.TrimSpace(requested)
	if requested != "" {
		if contains(available, requested) {
			return requested, nil
		}
		return "", nivierrors.InvalidModel("selection.resolve", requested)
	}

	grouped := Group(models, "")
	if len(grouped.Recommended) > 0 {
		return grouped.Recommended[0], nil
	}

	return available[0], nil
}

func Group(models []provider.Model, active string) GroupedModels {
	grouped := GroupedModels{
		Active: strings.TrimSpace(active),
	}

	available := uniqueSortedIDs(models)
	if len(available) == 0 {
		return grouped
	}

	index := make(map[string]struct{}, len(available))
	for _, modelID := range available {
		index[modelID] = struct{}{}
	}

	recommended := make(map[string]struct{}, len(recommendedOrder))
	for _, modelID := range recommendedOrder {
		if _, ok := index[modelID]; !ok || modelID == grouped.Active {
			continue
		}
		grouped.Recommended = append(grouped.Recommended, modelID)
		recommended[modelID] = struct{}{}
	}

	for _, modelID := range available {
		if modelID == grouped.Active {
			continue
		}
		if _, ok := recommended[modelID]; ok {
			continue
		}
		grouped.Others = append(grouped.Others, modelID)
	}

	return grouped
}

func Contains(models []provider.Model, target string) bool {
	return contains(uniqueSortedIDs(models), strings.TrimSpace(target))
}

func uniqueSortedIDs(models []provider.Model) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}

		if _, ok := seen[modelID]; ok {
			continue
		}

		seen[modelID] = struct{}{}
		ids = append(ids, modelID)
	}

	sort.Strings(ids)

	return ids
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
