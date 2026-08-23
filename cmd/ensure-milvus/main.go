package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
)

func main() {
	collection := "fluxsearch_default"
	if v := os.Getenv("FLUXSEARCH_MILVUS_COLLECTION"); v != "" {
		collection = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := config.Load()
	idx := milvusstore.DefaultIndexConfig()
	store, err := milvusstore.NewStore(ctx, cfg, idx)
	if err != nil {
		log.Fatalf("connect milvus: %v", err)
	}
	defer store.Close()

	if err := store.EnsureCollection(ctx, collection); err != nil {
		log.Fatalf("ensure collection: %v", err)
	}

	n, err := store.Stats(ctx, collection)
	if err != nil {
		log.Printf("stats: %v", err)
	}
	fmt.Printf("OK collection=%s dim=%d rows=%d\n", collection, store.VectorDim(), n)
}
