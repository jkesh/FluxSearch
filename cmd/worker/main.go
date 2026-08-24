package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stores := bootstrap.InitWorkerStores(ctx)
	defer stores.Close()

	log.Printf("fluxsearch-worker started (import + reindex queues)")
	stores.StartWorker(ctx)

	<-ctx.Done()
	log.Printf("fluxsearch-worker shutting down")
}
