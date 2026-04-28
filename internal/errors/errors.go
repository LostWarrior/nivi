package nivierrors

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Kind string

const (
	KindConfig       Kind = "config"
	KindValidation   Kind = "validation"
	KindAuth         Kind = "auth"
	KindInvalidModel Kind = "invalid_model"
	KindNetwork      Kind = "network"
	KindTimeout      Kind = "timeout"
	KindUnavailable  Kind = "unavailable"
	KindProtocol     Kind = "protocol"
	KindAPI          Kind = "api"
)

type Error struct {
	Kind    Kind
	Op      string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	message := strings.TrimSpace(RedactSecrets(e.Message))
	if e.Op == "" {
		return message
	}
	if message == "" {
		return e.Op
	}
	return e.Op + ": " + message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(kind Kind, message string) error {
	return &Error{
		Kind:    kind,
		Message: message,
	}
}

func Wrap(kind Kind, message string, cause error) error {
	return &Error{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}

func withOp(kind Kind, op, message string, cause error) error {
	return &Error{
		Kind:    kind,
		Op:      op,
		Message: message,
		Cause:   cause,
	}
}

func Config(op, message string) error {
	return withOp(KindConfig, op, message, nil)
}

func Validation(op, message string) error {
	return withOp(KindValidation, op, message, nil)
}

func MissingAPIKey(op string) error {
	return withOp(
		KindConfig,
		op,
		`missing API key. Add export NVIDIA_API_KEY="nvapi-your-key-here" to your shell profile and restart your shell.`,
		nil,
	)
}

func Auth(op string) error {
	return withOp(
		KindAuth,
		op,
		"NVIDIA API authentication failed. Verify NVIDIA_API_KEY and try again.",
		nil,
	)
}

func InvalidModel(op, model string) error {
	return withOp(
		KindInvalidModel,
		op,
		fmt.Sprintf(`model %q is not available to this API key. Run "nivi models" to choose a valid model.`, model),
		nil,
	)
}

func Network(op string, cause error) error {
	return withOp(
		KindNetwork,
		op,
		"network request failed while contacting the NVIDIA API. Check connectivity and NIVI_BASE_URL.",
		cause,
	)
}

func Timeout(op string, cause error) error {
	return withOp(
		KindTimeout,
		op,
		"NVIDIA API request timed out. Check connectivity and try again.",
		cause,
	)
}

func Unavailable(op, message string, cause error) error {
	return withOp(KindUnavailable, op, message, cause)
}

func Protocol(op, message string, cause error) error {
	return withOp(KindProtocol, op, message, cause)
}

func API(op, message string, cause error) error {
	return withOp(KindAPI, op, message, cause)
}

func SafeMessage(err error) string {
	if err == nil {
		return ""
	}

	var typed *Error
	if errors.As(err, &typed) {
		return typed.Error()
	}
	return RedactSecrets(err.Error())
}

func IsKind(err error, kind Kind) bool {
	var typed *Error
	if !errors.As(err, &typed) {
		return false
	}
	return typed.Kind == kind
}

var redactPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)([^\s,;]+)`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bbearer\s+([^\s,;]+)`),
		replacement: `Bearer [REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`\bnvapi-[A-Za-z0-9._-]+\b`),
		replacement: `[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b(NVIDIA_API_KEY|NGC_API_KEY)\s*=\s*(['"]?)[^'"\s]+(['"]?)`),
		replacement: `${1}=${2}[REDACTED]${3}`,
	},
}

func RedactSecrets(input string) string {
	redacted := input
	for _, pattern := range redactPatterns {
		redacted = pattern.pattern.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}
