package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultNetworkProbeURL = "https://api.telegram.org"

// buildPingMessage 构建 /ping 命令的响应文本
func (b *Bot) buildPingMessage(ctx context.Context) string {
	lines := []string{"🏓 Pong!"}

	if !b.startTime.IsZero() {
		uptime := time.Since(b.startTime)
		lines = append(lines, fmt.Sprintf("⏱ 运行时间: %s", formatDuration(uptime)))
	}

	if b.workerPool != nil {
		stats := b.workerPool.Stats()
		lines = append(lines, fmt.Sprintf("🛠 工作池: %d 个协程，队列 %d/%d", stats.Workers, stats.QueueLength, stats.QueueCapacity))
	}

	if b.db != nil {
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if err := b.db.Client().Ping(dbCtx, nil); err != nil {
			lines = append(lines, fmt.Sprintf("🗄 数据库: ⚠️ %v", err))
		} else {
			lines = append(lines, "🗄 数据库: ✅ 正常")
		}
	}

	networkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	latency, statusCode, err := probeNetwork(networkCtx, defaultNetworkProbeURL)
	if err != nil {
		lines = append(lines, fmt.Sprintf("🌐 网络: ⚠️ 测速失败 (%v)", err))
	} else {
		lines = append(lines, fmt.Sprintf("🌐 网络延迟: %s（%s，HTTP %d）", latency.Round(time.Millisecond), defaultNetworkProbeURL, statusCode))
	}

	return strings.Join(lines, "\n")
}

// probeNetwork 测试与指定地址的网络连通性，返回耗时与状态码
func probeNetwork(ctx context.Context, target string) (time.Duration, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, err
	}

	client := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return time.Since(start), resp.StatusCode, nil
}

// formatDuration 将持续时间格式化为人类可读的字符串
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	d = d.Round(time.Second)

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d小时", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d分钟", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d秒", seconds))
	}

	return strings.Join(parts, " ")
}
