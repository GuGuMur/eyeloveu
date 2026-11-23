package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"eyeloveu/internal/reminder"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := reminder.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
