package logger

import (
    "fmt"
    "os"

    "github.com/fatih/color"
    "github.com/mattn/go-isatty"
    "myapp/internal/theme"
)

var (
    userC     *color.Color
    assistantC *color.Color
    systemC  *color.Color
)

func init() {
    scheme := theme.Detect()
    switch scheme {
    case "dark":
        userC = color.New(color.FgHiGreen)
        assistantC = color.New(color.FgHiBlue)
        systemC = color.New(color.FgHiYellow)
    default: // light
        userC = color.New(color.FgGreen)
        assistantC = color.New(color.FgBlue)
        systemC = color.New(color.FgYellow)
    }
}

func User(msg string) {
    if isatty.IsTerminal(os.Stdout.Fd()) {
        userC.Println(msg)
    } else {
        fmt.Println(msg)
    }
}

func Assistant(msg string) {
    if isatty.IsTerminal(os.Stdout.Fd()) {
        assistantC.Println(msg)
    } else {
        fmt.Println(msg)
    }
}

func System(msg string) {
    if isatty.IsTerminal(os.Stdout.Fd()) {
        systemC.Println(msg)
    } else {
        fmt.Println(msg)
    }
}
