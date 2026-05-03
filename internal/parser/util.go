package parser

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"brokers-sync/internal/model"
)

func mapColumns(header []string, required []string) (map[string]int, error) {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing column %q", col)
		}
	}
	return idx, nil
}

func parseAnyTime(formats []string, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

func syntheticID(tx model.Transaction) string {
	key := fmt.Sprintf("%s|%s|%s|%s|%f|%f",
		tx.Broker, tx.Date.UTC().Format(time.RFC3339Nano),
		tx.Symbol, string(tx.Type), tx.Quantity, tx.Net)
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum[:8])
}
