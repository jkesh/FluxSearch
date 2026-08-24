package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/settings"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
)

func main() {
	recreate := flag.Bool("recreate", false, "drop and recreate the collection (use when vector dim changed)")
	flag.Parse()

	collection := "fluxsearch_default"
	if v := os.Getenv("FLUXSEARCH_MILVUS_COLLECTION"); v != "" {
		collection = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	settingsMgr := settings.NewManager()
	cfg := settingsMgr.ToConfig()
	idx := settingsMgr.MilvusIndexConfig()
	store, err := milvusstore.NewStore(ctx, cfg, idx)
	if err != nil {
		log.Fatalf("connect milvus: %v", err)
	}
	defer store.Close()

	if *recreate {
		if err := store.RecreateCollection(ctx, collection); err != nil {
			log.Fatalf("recreate collection: %v", err)
		}
	} else if err := store.EnsureCollection(ctx, collection); err != nil {
		log.Fatalf("ensure collection: %v", err)
	}

	actualDim, err := store.CollectionVectorDim(ctx, collection)
	if err != nil {
		log.Printf("describe collection: %v", err)
		actualDim = store.VectorDim()
	}

	n, err := store.Stats(ctx, collection)
	if err != nil {
		log.Printf("stats: %v", err)
	}
	fmt.Printf("OK collection=%s configured_dim=%d actual_dim=%d rows=%d\n", collection, store.VectorDim(), actualDim, n)
	if actualDim > 0 && actualDim != store.VectorDim() {
		os.Exit(1)
	}
}
