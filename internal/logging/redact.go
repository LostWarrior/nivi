package logging

import (
	"strings"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

func Redact(input string) string {
	return strings.TrimSpace(nivierrors.RedactSecrets(input))
}
