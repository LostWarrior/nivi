package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

const (
	DefaultBaseURL      = "https://integrate.api.nvidia.com/v1"
	DefaultSystemPrompt = "You are a helpful CLI assistant."
	DefaultMaxTokens    = 2048
	DefaultLogLevel     = "info"
)

type Options struct {
	Model            string
	BaseURL          string
	SystemPrompt     string
	DisableStreaming bool
	MaxTokens        int
}

type State struct {
	APIKey           string
	APIKeySource     string
	BaseURL          string
	DefaultModel     string
	SystemPrompt     string
	StreamingEnabled bool
	MaxTokens        int
	LogLevel         string
}

func Load(options Options) (State, error) {
	state, err := resolve(options, os.LookupEnv)
	if err != nil {
		return State{}, err
	}
	if err := state.Validate(); err != nil {
		return State{}, err
	}
	return state, nil
}

func Resolve(options Options) State {
	state, _ := resolve(options, os.LookupEnv)
	return state
}

func (s State) Validate() error {
	if s.APIKey == "" {
		return nivierrors.MissingAPIKey("config.validate")
	}

	if err := ValidateBaseURL(s.BaseURL); err != nil {
		return err
	}

	if s.MaxTokens <= 0 {
		return nivierrors.Validation("config.validate", "max tokens must be greater than zero")
	}
	if !validLogLevel(s.LogLevel) {
		return nivierrors.Config(
			"config.validate",
			`NIVI_LOG_LEVEL must be one of "debug", "info", "warn", or "error".`,
		)
	}

	return nil
}

func ValidateBaseURL(rawURL string) error {
	_, err := NormalizeBaseURL(rawURL)
	return err
}

func NormalizeBaseURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		trimmed = DefaultBaseURL
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return "", nivierrors.Config(
			"config.validate_base_url",
			"invalid NIVI_BASE_URL; expected an HTTPS URL such as https://integrate.api.nvidia.com/v1",
		)
	}

	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	if parsedURL.Path == "" || parsedURL.Path == "/" {
		parsedURL.Path = "/v1"
	}

	return strings.TrimRight(parsedURL.String(), "/"), nil
}

func resolve(options Options, lookup func(string) (string, bool)) (State, error) {
	apiKeySource, apiKey := resolveAPIKey(lookup)

	streamingDisabled := options.DisableStreaming
	if value, ok := readEnv(lookup, "NIVI_DISABLE_STREAMING"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return State{}, nivierrors.Config(
				"config.resolve",
				`NIVI_DISABLE_STREAMING must be a boolean such as "true" or "false".`,
			)
		}
		streamingDisabled = streamingDisabled || parsed
	}

	maxTokens := options.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	baseURL := firstNonEmpty(
		strings.TrimSpace(options.BaseURL),
		envValue(lookup, "NIVI_BASE_URL"),
		DefaultBaseURL,
	)
	normalizedBaseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return State{}, err
	}

	systemPrompt := firstNonEmpty(
		strings.TrimSpace(options.SystemPrompt),
		envValue(lookup, "NIVI_SYSTEM_PROMPT"),
		DefaultSystemPrompt,
	)

	model := firstNonEmpty(
		strings.TrimSpace(options.Model),
		envValue(lookup, "NIVI_MODEL"),
	)

	logLevel := firstNonEmpty(strings.TrimSpace(envValue(lookup, "NIVI_LOG_LEVEL")), DefaultLogLevel)

	return State{
		APIKey:           apiKey,
		APIKeySource:     apiKeySource,
		BaseURL:          normalizedBaseURL,
		DefaultModel:     model,
		SystemPrompt:     systemPrompt,
		StreamingEnabled: !streamingDisabled,
		MaxTokens:        maxTokens,
		LogLevel:         strings.ToLower(logLevel),
	}, nil
}

func resolveAPIKey(lookup func(string) (string, bool)) (string, string) {
	if value := envValue(lookup, "NVIDIA_API_KEY"); value != "" {
		return "NVIDIA_API_KEY", value
	}

	if value := envValue(lookup, "NGC_API_KEY"); value != "" {
		return "NGC_API_KEY", value
	}

	return "", ""
}

func validLogLevel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func envValue(lookup func(string) (string, bool), key string) string {
	value, _ := readEnv(lookup, key)
	return value
}

func readEnv(lookup func(string) (string, bool), key string) (string, bool) {
	value, ok := lookup(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
