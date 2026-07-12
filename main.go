package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Build A version string that can be set with -ldflags "-X main.Build=SOMEVERSION"
var Build string

const (
	appName string = "currency-converter"

	// ratesURL is Danmarks Nationalbank's daily exchange rate feed. It publishes
	// the official reference rates as DKK per 100 units of each foreign currency.
	ratesURL string = "https://www.nationalbanken.dk/api/currencyratesxml?lang=en"

	// cacheFile is the name of the cached feed inside the user cache directory.
	cacheFile string = "cconv/rates.xml"

	// cacheTTL throttles how often we re-check the feed when the cached rates are
	// not yet from today. Rates publish around 16:00 CET on business days, so a
	// short window catches the daily update without hammering the API on
	// weekends and holidays when nothing new is published.
	cacheTTL time.Duration = 3 * time.Hour
)

// exchangeRatesXML mirrors the structure of Nationalbanken's currency rate feed.
type exchangeRatesXML struct {
	DailyRates struct {
		Date       string `xml:"id,attr"`
		Currencies []struct {
			Code string `xml:"code,attr"`
			Desc string `xml:"desc,attr"`
			Rate string `xml:"rate,attr"`
		} `xml:"currency"`
	} `xml:"dailyrates"`
}

var (
	currencyFrom string
	currencyTo   []string
	amount       float64
	debug        bool
	noCache      bool

	rootCmd = &cobra.Command{
		Use:     appName,
		Short:   "Currency converter using official Danish exchange rates",
		Run:     runConverter,
		Version: Build,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&currencyFrom, "from", "f", "DKK", "Choose currency to convert from")
	rootCmd.PersistentFlags().StringSliceVarP(&currencyTo, "to", "t", []string{}, "Choose currency to convert to (eg. eur,dkk). Default is all.")
	rootCmd.PersistentFlags().Float64VarP(&amount, "amount", "a", 1, "Set amount to convert")
	rootCmd.PersistentFlags().BoolVarP(&debug, "verbose", "v", false, "Enable verbose mode")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Bypass the local cache and fetch fresh rates")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}

// runConverter contains the main application logic.
func runConverter(cmd *cobra.Command, args []string) {
	applyPositionalArgs(cmd, args)

	fmt.Println(strings.ToUpper(currencyFrom), amount)

	// Rates are DKK per 100 units of the given currency, keyed by currency code.
	rates, lastUpdate, err := fetchRates()
	if err != nil {
		log.Fatalf("Failed to fetch exchange rates: %v", err)
	}

	// DKK is the reference currency and is not part of the feed. Since rates are
	// quoted per 100 units, DKK itself is 100.
	rates["DKK"] = 100.0

	if debug {
		fmt.Println("---")
		fmt.Println("Finished fetching. Found", len(rates), "currencies.")
		fmt.Println("Exchange rates last updated:", lastUpdate)
		fmt.Println("---")
	}

	fromUpper := strings.ToUpper(currencyFrom)
	fromRate, ok := rates[fromUpper]
	if !ok {
		fmt.Printf("Error: The 'from' currency '%s' was not found.\n", currencyFrom)
		return
	}

	// Determine which currencies to convert to.
	var targetCurrencies []string
	if len(currencyTo) > 0 {
		// If --to is specified, use those.
		targetCurrencies = currencyTo
	} else {
		// If --to is not specified, gather all available currencies.
		for code := range rates {
			if code != fromUpper { // Don't convert a currency to itself.
				targetCurrencies = append(targetCurrencies, code)
			}
		}
		// Sort for consistent, readable output.
		sort.Strings(targetCurrencies)
	}

	// Loop through each requested 'to' currency and perform the conversion.
	for _, toCurrency := range targetCurrencies {
		toUpper := strings.ToUpper(toCurrency)

		toRate, ok := rates[toUpper]
		if !ok {
			fmt.Printf("Error: The 'to' currency '%s' not found, skipping...\n", toCurrency)
			continue
		}

		// Cross-currency conversion. Both rates share the same DKK-per-100-units
		// basis, so the per-100 factor cancels out: (amount * from_rate) / to_rate.
		finalAmount := (amount * fromRate) / toRate

		fmt.Println(toUpper, toFixed(finalAmount, 3))
	}
}

// applyPositionalArgs interprets positional arguments as "[amount] [from] [to...]".
// The leading argument is treated as the amount when it is numeric, otherwise as
// the 'from' currency. Any explicitly set flag (-a/-f/-t) takes precedence over the
// corresponding positional argument.
func applyPositionalArgs(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		return
	}

	// A numeric leading argument is the amount.
	if v, err := strconv.ParseFloat(args[0], 64); err == nil {
		if !cmd.Flags().Changed("amount") {
			amount = v
		}
		args = args[1:]
	}

	// The next argument is the 'from' currency.
	if len(args) > 0 {
		if !cmd.Flags().Changed("from") {
			currencyFrom = args[0]
		}
		args = args[1:]
	}

	// Any remaining arguments are the 'to' currencies.
	if len(args) > 0 && !cmd.Flags().Changed("to") {
		currencyTo = args
	}
}

// fetchRates returns the exchange rates keyed by currency code, the date they were
// published, and any error. It serves cached rates when possible and only hits the
// network when the cache is missing, stale, or bypassed with --no-cache.
func fetchRates() (map[string]float64, string, error) {
	path := cachePath()

	// Serve from cache when it is still current.
	if path != "" && !noCache {
		if body, err := os.ReadFile(path); err == nil {
			if rates, date, err := parseRates(body); err == nil && cacheIsFresh(path, date) {
				if debug {
					fmt.Printf("Using cached exchange rates (dated %s) from %s\n", date, path)
				}
				return rates, date, nil
			}
		}
	}

	// Download fresh rates.
	body, err := downloadRates()
	if err != nil {
		// Fall back to a stale cache so a network hiccup doesn't break the tool.
		if path != "" {
			if stale, rerr := os.ReadFile(path); rerr == nil {
				if rates, date, perr := parseRates(stale); perr == nil {
					if debug {
						fmt.Printf("Download failed (%v); using stale cache dated %s.\n", err, date)
					}
					return rates, date, nil
				}
			}
		}
		return nil, "", err
	}

	if path != "" {
		writeCache(path, body)
	}
	return parseRates(body)
}

// cacheIsFresh reports whether a cache holding rates dated `date` can be reused.
// Today's rates are the newest that can exist, so they are always fresh; older
// rates are reused only until the cache file passes its TTL, which throttles
// rechecks when no newer rates have been published yet (weekends, holidays,
// before the ~16:00 CET publish time).
func cacheIsFresh(path, date string) bool {
	if date == today() {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && time.Since(info.ModTime()) < cacheTTL
}

// downloadRates fetches the raw feed body from Nationalbanken.
func downloadRates() ([]byte, error) {
	resp, err := http.Get(ratesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s from %s", resp.Status, ratesURL)
	}
	return io.ReadAll(resp.Body)
}

// parseRates decodes the feed body into rates keyed by currency code plus the
// publication date.
func parseRates(body []byte) (map[string]float64, string, error) {
	// The feed is served with a UTF-8 BOM, which encoding/xml rejects.
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})

	var data exchangeRatesXML
	if err := xml.Unmarshal(body, &data); err != nil {
		return nil, "", err
	}

	rates := make(map[string]float64)
	for _, cur := range data.DailyRates.Currencies {
		rate, err := strconv.ParseFloat(cur.Rate, 64)
		if err != nil {
			if debug {
				fmt.Printf("Could not parse rate for %s: %v\n", cur.Code, err)
			}
			continue
		}
		rates[strings.ToUpper(cur.Code)] = rate
	}

	if len(rates) == 0 {
		return nil, "", fmt.Errorf("no exchange rates found in feed")
	}
	return rates, data.DailyRates.Date, nil
}

// cachePath returns the path to the cached feed, or "" if no cache dir is available.
func cachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(cacheFile))
}

// writeCache persists the raw feed body to disk. Failures are non-fatal: the tool
// still works without a cache, just without its speed-up.
func writeCache(path string, body []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		if debug {
			fmt.Printf("Could not create cache dir: %v\n", err)
		}
		return
	}
	if err := os.WriteFile(path, body, 0o644); err != nil && debug {
		fmt.Printf("Could not write cache: %v\n", err)
	}
}

// today returns the current date in Copenhagen (where the rates are set), so the
// cache freshness check lines up with the feed's publication date.
func today() string {
	if loc, err := time.LoadLocation("Europe/Copenhagen"); err == nil {
		return time.Now().In(loc).Format("2006-01-02")
	}
	return time.Now().Format("2006-01-02")
}

// --- Helper Functions ---
func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
