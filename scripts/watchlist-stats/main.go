// watchlist-stats prints usage statistics for the watchlist table: totals, the
// most tracked symbols, and every portfolio (by its hashed key) with its stock
// count, sorted by the most recently added stock.
//
// It scans the whole table. The table is provisioned at 1 read unit and shared
// with both live apps, so a large scan is throttled and retried by the SDK and
// can briefly slow reads for users. Run it occasionally, not on a schedule.
//
//	go run ./scripts/watchlist-stats -profile hackmajoris-aws
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// row is the subset of attributes the scan fetches. Portfolio keys start with
// "P#"; SK is "META" for the portfolio record and "W#<SYMBOL>" per stock. The
// code cap counter lives at SYSTEM#codes / COUNTER.
type row struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	AddedAt   int64  `dynamodbav:"addedAt"`
	CreatedAt int64  `dynamodbav:"createdAt"`
	LastSeen  int64  `dynamodbav:"lastSeen"`
	// LastAccess is stamped when a list is opened, at most hourly. Lists not
	// opened since the field was introduced have none.
	LastAccess int64 `dynamodbav:"lastAccess"`
	Created    int64 `dynamodbav:"created"`
}

type account struct {
	Key        string
	Stocks     int
	LastAdded  time.Time // zero when the list is empty
	LastSymbol string
	Created    time.Time
	LastSeen   time.Time // last edit
	LastAccess time.Time // last opened; zero for lists not opened since tracking began
}

type stats struct {
	Accounts     []account // sorted: most recent addition first, empty lists last
	TotalStocks  int
	Empty        int
	CodesCreated int64 // from the cap counter; includes codes whose lists expired
	Symbols      map[string]int
}

func main() {
	table := flag.String("table", "brokers-sync-watchlist", "DynamoDB table name")
	region := flag.String("region", "eu-central-1", "AWS region")
	profile := flag.String("profile", "", "AWS profile (default: standard credential chain)")
	top := flag.Int("top", 10, "how many of the most tracked symbols to show")
	flag.Parse()

	ctx := context.Background()
	opts := []func(*config.LoadOptions) error{config.WithRegion(*region)}
	if *profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(*profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	rows, err := scan(ctx, dynamodb.NewFromConfig(cfg), *table)
	if err != nil {
		log.Fatalf("scan %s: %v", *table, err)
	}
	print(os.Stdout, aggregate(rows), *top)
}

func scan(ctx context.Context, db *dynamodb.Client, table string) ([]row, error) {
	p := dynamodb.NewScanPaginator(db, &dynamodb.ScanInput{
		TableName: aws.String(table),
		// Only what the report needs: notes and targets are never read.
		ProjectionExpression:     aws.String("PK, SK, addedAt, createdAt, lastSeen, lastAccess, #c"),
		ExpressionAttributeNames: map[string]string{"#c": "created"},
	})
	var rows []row
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		var batch []row
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}

func aggregate(rows []row) stats {
	s := stats{Symbols: map[string]int{}}
	byKey := map[string]*account{}
	get := func(pk string) *account {
		a := byKey[pk]
		if a == nil {
			a = &account{Key: pk}
			byKey[pk] = a
		}
		return a
	}

	for _, r := range rows {
		switch {
		case r.PK == "SYSTEM#codes":
			s.CodesCreated = r.Created
		case !strings.HasPrefix(r.PK, "P#"):
			continue
		case r.SK == "META":
			a := get(r.PK)
			a.Created = unix(r.CreatedAt)
			a.LastSeen = unix(r.LastSeen)
			a.LastAccess = unix(r.LastAccess)
		case strings.HasPrefix(r.SK, "W#"):
			a := get(r.PK)
			sym := strings.TrimPrefix(r.SK, "W#")
			a.Stocks++
			s.TotalStocks++
			s.Symbols[sym]++
			if t := unix(r.AddedAt); t.After(a.LastAdded) {
				a.LastAdded, a.LastSymbol = t, sym
			}
		}
	}

	for _, a := range byKey {
		if a.Stocks == 0 {
			s.Empty++
		}
		s.Accounts = append(s.Accounts, *a)
	}
	// Most recent addition first. Empty lists have no addition, so they go
	// last, newest portfolio first; the key breaks any remaining tie so the
	// output is stable between runs.
	sort.Slice(s.Accounts, func(i, j int) bool {
		a, b := s.Accounts[i], s.Accounts[j]
		if !a.LastAdded.Equal(b.LastAdded) {
			return a.LastAdded.After(b.LastAdded)
		}
		if !a.Created.Equal(b.Created) {
			return a.Created.After(b.Created)
		}
		return a.Key < b.Key
	})
	return s
}

// unix reads a stored timestamp. The server writes seconds; a value too large
// to be seconds is treated as milliseconds so a stray client value still sorts.
func unix(v int64) time.Time {
	switch {
	case v == 0:
		return time.Time{}
	case v > 1e12:
		return time.UnixMilli(v)
	default:
		return time.Unix(v, 0)
	}
}

func print(w io.Writer, s stats, top int) {
	n := len(s.Accounts)
	fmt.Fprintf(w, "Accounts        %d  (%d with stocks, %d empty)\n", n, n-s.Empty, s.Empty)
	fmt.Fprintf(w, "Codes created   %d  (cap counter; includes expired lists)\n", s.CodesCreated)
	fmt.Fprintf(w, "Stocks tracked  %d  (%d distinct symbols)\n", s.TotalStocks, len(s.Symbols))
	if n-s.Empty > 0 {
		fmt.Fprintf(w, "Avg per list    %.1f  (lists with stocks only)\n", float64(s.TotalStocks)/float64(n-s.Empty))
	}

	if len(s.Symbols) > 0 {
		fmt.Fprintf(w, "\nMost tracked\n")
		syms := make([]string, 0, len(s.Symbols))
		for sym := range s.Symbols {
			syms = append(syms, sym)
		}
		sort.Slice(syms, func(i, j int) bool {
			if s.Symbols[syms[i]] != s.Symbols[syms[j]] {
				return s.Symbols[syms[i]] > s.Symbols[syms[j]]
			}
			return syms[i] < syms[j]
		})
		for i, sym := range syms {
			if i == top {
				break
			}
			fmt.Fprintf(w, "  %-12s %d\n", sym, s.Symbols[sym])
		}
	}

	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT (hashed)\tSTOCKS\tLAST ADDED\tSYMBOL\tCREATED\tLAST EDIT\tLAST ACCESS")
	for _, a := range s.Accounts {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n", a.Key, a.Stocks, day(a.LastAdded), orDash(a.LastSymbol), day(a.Created), day(a.LastSeen), day(a.LastAccess))
	}
	_ = tw.Flush()
}

func day(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
