package theme

import (
    "os"
    "regexp"
)

func Detect() string {
    // 1. Check env var first
    if v := os.Getenv("NV_COLOR_SCHEME"); v != "" {
        return v
    }
    // 2. Default to light on non‑TTY / terminal that supports 256 colours
    if !isatty.IsTerminal(os.Stdout.Fd()) {
        return "light"
    }
    // 3. Try to read $COLORTERM or $TERM
    term := os.Getenv("TERM")
    if match, _ := regexp.MatchString(`256color`, term); match {
        return "light"
    }
    // For this prototype, default to light
    return "light"
}
