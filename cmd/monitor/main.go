package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/fluxsearch/fluxsearch/internal/health"
	"github.com/fluxsearch/fluxsearch/internal/monitor"
	"github.com/gin-gonic/gin"
)

func main() {
	addr := os.Getenv("FLUXSEARCH_MONITOR_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	cfg := config.Load()
	collector := monitor.NewCollector(cfg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "fluxsearch-monitor"})
	})

	r.GET("/api/v1/status", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()

		report := collector.Collect(ctx)
		code := http.StatusOK
		if report.Overall == health.StatusDown {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, report)
	})

	log.Printf("fluxsearch-monitor listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
