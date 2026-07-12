package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// sampleFeed is a trimmed copy of Nationalbanken's feed, including the UTF-8 BOM
// the real endpoint serves.
const sampleFeed = "\xEF\xBB\xBF" + `<?xml version="1.0" encoding="utf-8"?>` +
	`<exchangerates type="Exchange rates" author="Danmarks Nationalbank" refcur="DKK" refamt="1">` +
	`<dailyrates id="2026-07-10">` +
	`<currency code="EUR" desc="Euro" rate="747.51" />` +
	`<currency code="USD" desc="US dollars" rate="653.99" />` +
	`<currency code="JPY" desc="Japanese yen" rate="4.0402" />` +
	`</dailyrates></exchangerates>`

func TestConvert(t *testing.T) {
	tests := []struct {
		name             string
		amount, from, to float64
		want             float64
	}{
		{"eur to dkk", 1, 747.51, 100, 7.4751},
		{"same currency is identity", 5, 653.99, 653.99, 5},
		{"scales linearly with amount", 10, 747.51, 100, 74.751},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convert(tt.amount, tt.from, tt.to); !almostEqual(got, tt.want) {
				t.Errorf("convert(%v, %v, %v) = %v, want %v", tt.amount, tt.from, tt.to, got, tt.want)
			}
		})
	}

	// Cross rate: 100 DKK -> USD with USD at 653.99 DKK/100.
	if got := toFixed(convert(100, 100, 653.99), 3); got != 15.291 {
		t.Errorf("100 DKK -> USD = %v, want 15.291", got)
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		in   float64
		want int
	}{
		{2.4, 2}, {2.5, 3}, {2.6, 3}, {-2.4, -2}, {-2.5, -3}, {0, 0},
	}
	for _, tt := range tests {
		if got := round(tt.in); got != tt.want {
			t.Errorf("round(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestToFixed(t *testing.T) {
	tests := []struct {
		num       float64
		precision int
		want      float64
	}{
		{13.3781, 3, 13.378},
		{0.13378, 3, 0.134},
		{7.4751, 3, 7.475},
		{-1.2349, 3, -1.235},
		{5, 3, 5},
	}
	for _, tt := range tests {
		if got := toFixed(tt.num, tt.precision); got != tt.want {
			t.Errorf("toFixed(%v, %d) = %v, want %v", tt.num, tt.precision, got, tt.want)
		}
	}
}

func TestParseRates(t *testing.T) {
	rates, date, err := parseRates([]byte(sampleFeed))
	if err != nil {
		t.Fatalf("parseRates returned error: %v", err)
	}
	if date != "2026-07-10" {
		t.Errorf("date = %q, want %q", date, "2026-07-10")
	}
	if len(rates) != 3 {
		t.Errorf("len(rates) = %d, want 3", len(rates))
	}
	if got := rates["EUR"]; got != 747.51 {
		t.Errorf("EUR rate = %v, want 747.51", got)
	}
	if got := rates["JPY"]; got != 4.0402 {
		t.Errorf("JPY rate = %v, want 4.0402", got)
	}
}

func TestParseRatesSkipsUnparseableRate(t *testing.T) {
	feed := `<exchangerates><dailyrates id="2026-07-10">` +
		`<currency code="EUR" desc="Euro" rate="747.51" />` +
		`<currency code="BAD" desc="Bad" rate="n/a" />` +
		`</dailyrates></exchangerates>`
	rates, _, err := parseRates([]byte(feed))
	if err != nil {
		t.Fatalf("parseRates returned error: %v", err)
	}
	if len(rates) != 1 {
		t.Errorf("len(rates) = %d, want 1 (bad rate skipped)", len(rates))
	}
	if _, ok := rates["BAD"]; ok {
		t.Error("BAD currency should have been skipped")
	}
}

func TestParseRatesErrors(t *testing.T) {
	t.Run("empty feed", func(t *testing.T) {
		feed := `<exchangerates><dailyrates id="2026-07-10"></dailyrates></exchangerates>`
		if _, _, err := parseRates([]byte(feed)); err == nil {
			t.Error("expected error for feed with no currencies")
		}
	})
	t.Run("malformed xml", func(t *testing.T) {
		if _, _, err := parseRates([]byte("not xml <<<")); err == nil {
			t.Error("expected error for malformed xml")
		}
	})
}

func TestApplyPositionalArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flags    map[string]string // flags set explicitly before parsing positionals
		wantAmt  float64
		wantFrom string
		wantTo   []string
	}{
		{"amount from to", []string{"100", "dkk", "eur"}, nil, 100, "dkk", []string{"eur"}},
		{"amount from multi-to", []string{"100", "dkk", "eur", "gbp"}, nil, 100, "dkk", []string{"eur", "gbp"}},
		{"currency-first omits amount", []string{"dkk", "eur"}, nil, 1, "dkk", []string{"eur"}},
		{"amount and from only", []string{"100", "usd"}, nil, 100, "usd", []string{}},
		{"no args keeps defaults", nil, nil, 1, "DKK", []string{}},
		{"amount flag overrides positional", []string{"100", "dkk", "eur"}, map[string]string{"amount": "5"}, 5, "dkk", []string{"eur"}},
		{"to flag overrides positional", []string{"100", "dkk", "eur"}, map[string]string{"to": "gbp"}, 100, "dkk", []string{"gbp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newParseCmd() // resets globals to defaults via flag registration
			for name, val := range tt.flags {
				if err := cmd.Flags().Set(name, val); err != nil {
					t.Fatalf("setting flag %s: %v", name, err)
				}
			}

			applyPositionalArgs(cmd, tt.args)

			if amount != tt.wantAmt {
				t.Errorf("amount = %v, want %v", amount, tt.wantAmt)
			}
			if currencyFrom != tt.wantFrom {
				t.Errorf("from = %q, want %q", currencyFrom, tt.wantFrom)
			}
			if !reflect.DeepEqual(currencyTo, tt.wantTo) {
				t.Errorf("to = %#v, want %#v", currencyTo, tt.wantTo)
			}
		})
	}
}

// newParseCmd builds a command with the flags bound to the package globals, which
// also resets those globals to their defaults.
func newParseCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "cconv", Run: func(*cobra.Command, []string) {}}
	cmd.Flags().StringVarP(&currencyFrom, "from", "f", "DKK", "")
	cmd.Flags().StringSliceVarP(&currencyTo, "to", "t", []string{}, "")
	cmd.Flags().Float64VarP(&amount, "amount", "a", 1, "")
	return cmd
}

func TestCacheIsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rates.xml")
	if err := os.WriteFile(path, []byte(sampleFeed), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("today's date is always fresh", func(t *testing.T) {
		// Even with an old modtime, today's rates are the newest possible.
		old := time.Now().Add(-24 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		if !cacheIsFresh(path, today()) {
			t.Error("cache dated today should be fresh regardless of modtime")
		}
	})

	t.Run("old date within TTL is fresh", func(t *testing.T) {
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
		if !cacheIsFresh(path, "2000-01-01") {
			t.Error("recently fetched cache should be fresh even with an old feed date")
		}
	})

	t.Run("old date past TTL is stale", func(t *testing.T) {
		old := time.Now().Add(-2 * cacheTTL)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		if cacheIsFresh(path, "2000-01-01") {
			t.Error("old feed date past the TTL should be stale")
		}
	})

	t.Run("missing file is stale", func(t *testing.T) {
		if cacheIsFresh(filepath.Join(dir, "nope.xml"), "2000-01-01") {
			t.Error("missing cache file should be stale")
		}
	})
}

func TestCurrencyCodes(t *testing.T) {
	t.Run("falls back to static list without a cache", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir()) // empty cache dir
		got := currencyCodes()
		if !reflect.DeepEqual(got, fallbackCurrencyCodes) {
			t.Errorf("without cache, got %v, want fallback list", got)
		}
	})

	t.Run("reads from cache when present", func(t *testing.T) {
		cacheDir := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", cacheDir)
		if err := os.MkdirAll(filepath.Join(cacheDir, "cconv"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cacheDir, "cconv", "rates.xml"), []byte(sampleFeed), 0o644); err != nil {
			t.Fatal(err)
		}
		got := currencyCodes()
		want := []string{"dkk", "eur", "jpy", "usd"} // lowercase, sorted, incl. DKK
		if !reflect.DeepEqual(got, want) {
			t.Errorf("from cache got %v, want %v", got, want)
		}
	})
}

func TestCompleteCurrencies(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	codes, directive := completeCurrencies(nil, nil, "")
	if len(codes) == 0 {
		t.Error("expected currency suggestions, got none")
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestFetchRatesServesFreshCache(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	if err := os.MkdirAll(filepath.Join(cacheDir, "cconv"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A cache dated today is always fresh, so fetchRates must not touch the network.
	feed := `<exchangerates><dailyrates id="` + today() + `">` +
		`<currency code="EUR" desc="Euro" rate="747.51" /></dailyrates></exchangerates>`
	if err := os.WriteFile(filepath.Join(cacheDir, "cconv", "rates.xml"), []byte(feed), 0o644); err != nil {
		t.Fatal(err)
	}

	noCache = false
	rates, date, err := fetchRates()
	if err != nil {
		t.Fatalf("fetchRates returned error: %v", err)
	}
	if date != today() {
		t.Errorf("date = %q, want today %q", date, today())
	}
	if rates["EUR"] != 747.51 {
		t.Errorf("EUR rate = %v, want 747.51", rates["EUR"])
	}
}

// TestLiveFeedFormat is a canary against Nationalbanken's public feed: it verifies
// the real endpoint is reachable and still matches the structure parseRates
// expects. It fails loudly if the feed is unavailable (non-200 / unreachable) or
// its schema has changed. Run `go test -short` to skip it (e.g. when offline).
func TestLiveFeedFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live feed test in short mode")
	}

	body, err := downloadRates()
	if err != nil {
		t.Fatalf("live feed is not available: %v", err)
	}

	rates, date, err := parseRates(body)
	if err != nil {
		t.Fatalf("live feed no longer parses (schema may have changed): %v", err)
	}

	if _, err := time.Parse("2006-01-02", date); err != nil {
		t.Errorf("feed date %q is not in the expected YYYY-MM-DD format: %v", date, err)
	}

	// The feed has published ~30 currencies for years; a large drop signals a
	// structural change worth investigating.
	if len(rates) < 20 {
		t.Errorf("live feed returned only %d currencies, expected >= 20", len(rates))
	}

	// Major currencies must be present with sane, positive rates.
	for _, code := range []string{"EUR", "USD", "GBP"} {
		rate, ok := rates[code]
		if !ok {
			t.Errorf("expected currency %s missing from live feed", code)
			continue
		}
		if rate <= 0 {
			t.Errorf("%s rate = %v, expected a positive value", code, rate)
		}
	}
}

func almostEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	return d < eps && d > -eps
}
