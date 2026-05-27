package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/allofher/agent-kate-reid/internal/agent"
	"github.com/allofher/agent-kate-reid/internal/config"
	"github.com/allofher/agent-kate-reid/internal/provider"
	"github.com/allofher/agent-kate-reid/internal/strudel"
)

func main() {
	cfgPath := flag.String("config", "kate.json", "path to kate config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	p, err := provider.FromConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building provider: %v\n", err)
		os.Exit(1)
	}

	strudelClient := strudel.New(cfg.StrudelURL)
	kate := agent.New(strudelClient, p)

	log.Printf("kate-reid ready  provider=%s  model=%s  strudel=%s",
		cfg.Provider, kate.Provider.Model(), cfg.StrudelURL)
}
