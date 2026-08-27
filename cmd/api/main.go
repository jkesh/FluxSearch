package main

import (
	"log"
	"os"

	"github.com/fluxsearch/fluxsearch/internal/api"
)

func main() {
	addr := os.Getenv("FLUXSEARCH_API_ADDR")
	if addr == "" {
		if port := os.Getenv("FLUXSEARCH_API_PORT"); port != "" {
			addr = ":" + port
		} else {
			addr = ":8080"
		}
	}

	router := api.NewRouter()
	log.Printf("fluxsearch-api listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
