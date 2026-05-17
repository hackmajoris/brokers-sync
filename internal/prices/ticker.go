package prices

import "strings"

// yahooOverrides maps internal symbols to their Yahoo Finance tickers,
// overriding the currency-based suffix rules below.
var yahooOverrides = map[string]string{}

// YahooTicker returns the Yahoo Finance ticker for a given symbol and currency.
// Rules (applied in order):
//  1. Static overrides take precedence.
//  2. If the symbol already contains "." it carries its own exchange suffix — leave it unchanged.
//  3. RON-denominated symbols trade on the Bucharest Stock Exchange → append ".RO".
//  4. EUR-denominated symbols trade on Xetra → append ".DE".
//  5. Otherwise return the symbol unchanged.
func YahooTicker(symbol, currency string) string {
	if override, ok := yahooOverrides[symbol]; ok {
		return override
	}
	if strings.Contains(symbol, ".") {
		return symbol
	}
	switch currency {
	case "RON":
		return symbol + ".RO"
	case "EUR":
		return symbol + ".DE"
	}
	return symbol
}
