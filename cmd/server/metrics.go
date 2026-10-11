package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Metric names shared with the alarm in infra/watchlist_stack.go. Renaming
// either side silently breaks the alarm, so keep them in step.
const (
	metricNamespace    = "BrokersSync/Watchlist"
	metricCodesCreated = "CodesCreated"
)

// emitCodeCreated writes one CloudWatch Embedded Metric Format line. Lambda
// ships stdout to CloudWatch Logs, which turns the line into a metric with no
// SDK call and no extra IAM permission.
func emitCodeCreated(w io.Writer, now time.Time) {
	line, _ := json.Marshal(map[string]any{
		"_aws": map[string]any{
			"Timestamp": now.UnixMilli(),
			"CloudWatchMetrics": []map[string]any{{
				"Namespace":  metricNamespace,
				"Dimensions": [][]string{{"App"}},
				"Metrics":    []map[string]string{{"Name": metricCodesCreated, "Unit": "Count"}},
			}},
		},
		"App":              appWatchlist,
		metricCodesCreated: 1,
	})
	_, _ = fmt.Fprintln(w, string(line))
}
