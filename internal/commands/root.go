var (
    themeFlag string
)

func init() {
    rootCmd.Flags().StringVar(&themeFlag, "theme", "", "override colour scheme (dark|light)")
}

var postRun = func(cmd *cobra.Command, args []string) {
    if themeFlag != "" {
        // override the auto‑detect logic
        switch themeFlag {
        case "dark", "light":
            theme.Set(themeFlag)   // implement simple Set method using a global var
        }
    }
}
