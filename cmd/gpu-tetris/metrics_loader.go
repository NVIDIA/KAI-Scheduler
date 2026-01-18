package main

import (
	"bufio"
	"context"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// FetchQueueFairShares fetches queue_fair_share_gpu from Prometheus metrics endpoint
func FetchQueueFairShares(metricsURL string) (map[string]float64, error) {
	if metricsURL == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse lines like: kai_queue_fair_share_gpu{queue_name="default-queue"} 2.5
	result := make(map[string]float64)
	scanner := bufio.NewScanner(resp.Body)
	re := regexp.MustCompile(`kai_queue_fair_share_gpu\{queue_name="([^"]+)"\}\s+(\S+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			queueName := matches[1]
			value, _ := strconv.ParseFloat(matches[2], 64)
			result[queueName] = value
		}
	}
	return result, scanner.Err()
}
