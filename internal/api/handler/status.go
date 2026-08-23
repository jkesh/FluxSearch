package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/health"
	"github.com/fluxsearch/fluxsearch/internal/monitor"
	"github.com/gin-gonic/gin"
)

var statusHTTPClient = &http.Client{Timeout: 12 * time.Second}

func (h *Handler) SystemStatus(c *gin.Context) {
	monitorURL := h.stores.Settings.MonitorURL()

	if monitorURL != "" {
		report, err := fetchRemoteStatus(c.Request.Context(), monitorURL)
		if err == nil {
			report.Source = "remote"
			report.MonitorURL = monitorURL
			code := http.StatusOK
			if report.Overall == health.StatusDown {
				code = http.StatusServiceUnavailable
			}
			c.JSON(code, report)
			return
		}
		// 远程失败时返回错误信息 + 本地 API 状态
		c.JSON(http.StatusOK, monitor.Report{
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
			Overall:    health.StatusDegraded,
			Source:     "remote-failed",
			MonitorURL: monitorURL,
			Services: []health.ServiceCheck{
				{Name: "api", Label: "FluxSearch API", Category: "application", Status: health.StatusUp, Message: "running"},
				{Name: "monitor", Label: "远程 Monitor", Category: "application", Status: health.StatusDown, Endpoint: monitorURL, Message: err.Error()},
			},
		})
		return
	}

	// 无远程 URL 时走本地检查（开发直连模式）
	collector := monitor.NewCollector(h.stores.Config)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	report := collector.Collect(ctx)
	report.Source = "local"
	code := http.StatusOK
	if report.Overall == health.StatusDown {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, report)
}

func fetchRemoteStatus(ctx context.Context, url string) (monitor.Report, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return monitor.Report{}, err
	}

	resp, err := statusHTTPClient.Do(req)
	if err != nil {
		return monitor.Report{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return monitor.Report{}, err
	}

	var report monitor.Report
	if err := json.Unmarshal(body, &report); err != nil {
		return monitor.Report{}, fmt.Errorf("invalid response: %w", err)
	}
	return report, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
