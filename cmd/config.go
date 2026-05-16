package cmd

import (
	"fmt"
	"os"

	"github.com/zephel01/luna-go/internal/config"
)

// runConfig implements `luna config <show|init>`.
func runConfig(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "show", "":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			os.Exit(1)
		}
		config.Show(cfg)

	case "init":
		if config.Exists() {
			p, _ := config.Path()
			fmt.Fprintf(os.Stderr, "config file already exists: %s\n", p)
			fmt.Fprintln(os.Stderr, "use `luna config show` to view current settings.")
			os.Exit(1)
		}
		cfg, err := config.Load() // returns defaults when file is missing
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			os.Exit(1)
		}
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
			os.Exit(1)
		}
		p, _ := config.Path()
		fmt.Printf("✔ created %s\n", p)
		fmt.Println("edit it to set your preferred model, base_url, etc.")

	default:
		fmt.Fprintf(os.Stderr, "unknown config command %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: luna config <show|init>")
		os.Exit(1)
	}
}
