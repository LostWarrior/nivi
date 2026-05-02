package logging

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	themeDark  = "dark"
	themeLight = "light"
)

var (
	themeMu       sync.RWMutex
	themeOverride string
)

func SetTheme(name string) error {
	value := strings.ToLower(strings.TrimSpace(name))
	if value == "" {
		themeMu.Lock()
		themeOverride = ""
		themeMu.Unlock()
		return nil
	}
	if value != themeDark && value != themeLight {
		return fmt.Errorf(`invalid theme %q; expected "dark" or "light"`, name)
	}
	themeMu.Lock()
	themeOverride = value
	themeMu.Unlock()
	return nil
}

func WriteAssistant(output io.Writer, message string) (int, error) {
	return writeWithRoleColor(output, message, "assistant")
}

func writeWithRoleColor(output io.Writer, message, role string) (int, error) {
	if output == nil {
		output = io.Discard
	}
	if !supportsColor(output) {
		return fmt.Fprintln(output, message)
	}

	code := colorCode(resolveTheme(), role)
	if code == "" {
		return fmt.Fprintln(output, message)
	}
	return fmt.Fprintln(output, wrapANSI(message, code))
}

func resolveTheme() string {
	themeMu.RLock()
	override := themeOverride
	themeMu.RUnlock()
	if override != "" {
		return override
	}

	for _, key := range []string{"NIVI_THEME", "NV_COLOR_SCHEME"} {
		if value := normalizeTheme(os.Getenv(key)); value != "" {
			return value
		}
	}
	if inferred := inferThemeFromColorfgbg(os.Getenv("COLORFGBG")); inferred != "" {
		return inferred
	}
	return themeDark
}

func normalizeTheme(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case themeDark:
		return themeDark
	case themeLight:
		return themeLight
	default:
		return ""
	}
}

func inferThemeFromColorfgbg(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ";")
	if len(parts) == 0 {
		return ""
	}
	background := strings.TrimSpace(parts[len(parts)-1])
	code, err := strconv.Atoi(background)
	if err != nil {
		return ""
	}
	if code <= 6 {
		return themeDark
	}
	return themeLight
}

func supportsColor(output io.Writer) bool {
	if force := strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")); force != "" && force != "0" {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func colorCode(theme, role string) string {
	switch role {
	case "assistant":
		if theme == themeDark {
			return "94"
		}
		return "34"
	default:
		return ""
	}
}

func wrapANSI(input, code string) string {
	return "\x1b[" + code + "m" + input + "\x1b[0m"
}
