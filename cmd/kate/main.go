package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/allofher/agent-kate-reid/internal/agent"
	"github.com/allofher/agent-kate-reid/internal/config"
	"github.com/allofher/agent-kate-reid/internal/harness"
	"github.com/allofher/agent-kate-reid/internal/provider"
	"github.com/allofher/agent-kate-reid/internal/strudel"
	"github.com/allofher/agent-kate-reid/internal/tools"
)

func main() {
	cfgPath := flag.String("config", "kate.json", "path to kate config file")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	p, err := provider.FromConfig(cfg)
	if err != nil {
		return fmt.Errorf("building provider: %w", err)
	}

	systemPrompt := ""
	if cfg.SystemPromptPath != "" {
		data, err := os.ReadFile(cfg.SystemPromptPath)
		if err != nil {
			return fmt.Errorf("reading system prompt: %w", err)
		}
		systemPrompt = string(data)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	strudelClient := strudel.New(cfg.StrudelURL)
	if err := strudelClient.Connect(ctx); err != nil {
		return err
	}
	defer strudelClient.Close()

	queue := harness.NewQueue()
	scheduler := harness.NewScheduler(strudelClient, queue, harness.Config{
		Mode:       harness.TickMode(cfg.Harness.TickMode),
		Cycles:     cfg.Harness.TickCycles,
		Interval:   time.Duration(cfg.Harness.TickSeconds * float64(time.Second)),
		DefaultCPS: cfg.Harness.DefaultCPS,
	})
	registry := tools.New(queue, strudelClient)
	kate := agent.New(p, registry, scheduler.Reports(), systemPrompt)

	log.Printf("kate-reid joining the session  provider=%s  model=%s  strudel=%s  tick=%s",
		cfg.Provider, p.Model(), cfg.StrudelURL, cfg.Harness.TickMode)

	go scheduler.Run(ctx)
	err = kate.Run(ctx)

	// Leave the stage quietly: silence everything Kate was playing.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, stopErr := strudelClient.StopAll(shutdownCtx); stopErr != nil {
		log.Printf("shutdown: stop all: %v", stopErr)
	}
	log.Printf("kate-reid left the session")
	return err
}
