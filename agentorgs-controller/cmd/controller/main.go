package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/app"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load()
	application, err := app.New(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start AgentOrgs controller: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("agentorgs-controller is running")
	if err := application.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "controller exited: %v\n", err)
		os.Exit(1)
	}
}
