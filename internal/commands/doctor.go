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

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Details string `json:"details"`
}

type DoctorReport struct {
	Healthy      bool          `json:"healthy"`
	ActiveModel  string        `json:"active_model,omitempty"`
	StreamingTTY bool          `json:"streaming_tty"`
	Checks       []DoctorCheck `json:"checks"`
}

func RunDoctor(ctx context.Context, output io.Writer, state config.State, jsonOutput bool) error {
	if output == nil {
		output = os.Stderr
	}

	report := DoctorReport{
		StreamingTTY: state.StreamingEnabled && isTTY(),
		Checks: []DoctorCheck{
			{
				Name:    "api_key",
				OK:      state.APIKey != "",
				Details: apiKeyDetails(state),
			},
		},
	}

	baseURLErr := config.ValidateBaseURL(state.BaseURL)
	report.Checks = append(report.Checks, DoctorCheck{
		Name:    "base_url",
		OK:      baseURLErr == nil,
		Details: baseURLDetails(state.BaseURL, baseURLErr),
	})

	if state.APIKey != "" && state.APIKeySource == "NGC_API_KEY" {
		report.Checks = append(report.Checks, DoctorCheck{
			Name:    "api_key_source",
			OK:      true,
			Details: "using NGC_API_KEY compatibility fallback; prefer NVIDIA_API_KEY.",
		})
	}

	if reportHealthy(report.Checks) {
		client, err := provider.New(state)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Name:    "provider",
				OK:      false,
				Details: nivierrors.SafeMessage(err),
			})
		} else {
			models, listErr := client.ListModels(ctx)
			if listErr != nil {
				report.Checks = append(report.Checks, DoctorCheck{
					Name:    "models_api",
					OK:      false,
					Details: nivierrors.SafeMessage(listErr),
				})
			} else {
				report.Checks = append(report.Checks, DoctorCheck{
					Name:    "models_api",
					OK:      true,
					Details: fmt.Sprintf("connected successfully; %d model(s) available.", len(models)),
				})

				activeModel, resolveErr := selection.Resolve(state.DefaultModel, models)
				report.Checks = append(report.Checks, DoctorCheck{
					Name:    "active_model",
					OK:      resolveErr == nil,
					Details: activeModelDetails(activeModel, resolveErr),
				})
				if resolveErr == nil {
					report.ActiveModel = activeModel
				}
			}
		}
	}

	report.Healthy = reportHealthy(report.Checks)

	if jsonOutput {
		if err := json.NewEncoder(output).Encode(report); err != nil {
			return err
		}
	} else {
		for _, check := range report.Checks {
			status := "PASS"
			if !check.OK {
				status = "FAIL"
			}
			if _, err := fmt.Fprintf(output, "[%s] %s: %s\n", status, check.Name, check.Details); err != nil {
				return err
			}
		}
		if report.ActiveModel != "" {
			if _, err := fmt.Fprintf(output, "active_model: %s\n", report.ActiveModel); err != nil {
				return err
			}
		}
	}

	if report.Healthy {
		return nil
	}
	return nivierrors.Validation("commands.doctor", "doctor found one or more setup issues")
}

func apiKeyDetails(state config.State) string {
	if state.APIKey == "" {
		return `missing. Add export NVIDIA_API_KEY="nvapi-your-key-here" to your shell profile and restart your shell.`
	}
	if state.APIKeySource == "NGC_API_KEY" {
		return "loaded from NGC_API_KEY compatibility fallback."
	}
	return "loaded from NVIDIA_API_KEY."
}

func baseURLDetails(baseURL string, err error) string {
	if err != nil {
		return nivierrors.SafeMessage(err)
	}
	return baseURL
}

func activeModelDetails(activeModel string, err error) string {
	if err != nil {
		return nivierrors.SafeMessage(err)
	}
	return activeModel
}

func reportHealthy(checks []DoctorCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func isTTY() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
