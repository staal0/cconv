package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
	"github.com/spf13/cobra"
)

// Build A version string that can be set with -ldflags "-X main.Build=SOMEVERSION"
var Build string

const (
	appName string = "currency-converter"
)

// Rate holds the buy and sale values for a currency.
type Rate struct {
	Buy        float64
	Sale       float64
	LastUpdate string
}

var (
	currencyFrom string
	currencyTo   []string
	amount       float64
	debug        bool

	rootCmd = &cobra.Command{
		Use:     appName,
		Short:   "Simple fetch and convert currency tool",
		Run:     runConverter,
		Version: Build,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&currencyFrom, "from", "f", "DKK", "Choose currency to convert from")
	rootCmd.PersistentFlags().StringSliceVarP(&currencyTo, "to", "t", []string{}, "Choose currency to convert to (eg. eur,dkk). Default is all.")
	rootCmd.PersistentFlags().Float64VarP(&amount, "amount", "a", 1, "Set amount to convert")
	rootCmd.PersistentFlags().BoolVarP(&debug, "verbose", "v", false, "Enable verbose mode")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}

// runConverter contains the main application logic.
func runConverter(_ *cobra.Command, _ []string) {
	url := "https://www.nykredit.dk/dit-liv/valutakurser/noteringskurser/"
	fmt.Println(strings.ToUpper(currencyFrom), amount)

	c := colly.NewCollector()

	// exchangeRates will store all scraped currency data with the currency code as the key.
	exchangeRates := make(map[string]Rate)

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	// PHASEN 1: SCRAPING
	// This callback scrapes all the data from the table first.
	c.OnHTML("table > tbody > tr", func(el *colly.HTMLElement) {
		currencyCode := el.ChildText("td:nth-child(3)")
		if currencyCode == "" {
			return // Skip rows that aren't currency rows
		}

		saleStr := normalizeNumber(el.ChildText("td:nth-child(4)"))
		buyStr := normalizeNumber(el.ChildText("td:nth-child(5)"))
		lastUpdateStr := el.ChildText("td:nth-child(7)")

		sale, err := strconv.ParseFloat(saleStr, 64)
		if err != nil {
			if debug {
				fmt.Printf("Could not parse sale rate for %s: %v\n", currencyCode, err)
			}
			return
		}

		buy, err := strconv.ParseFloat(buyStr, 64)
		if err != nil {
			if debug {
				fmt.Printf("Could not parse buy rate for %s: %v\n", currencyCode, err)
			}
			return
		}

		// Store the parsed rates in our map.
		exchangeRates[currencyCode] = Rate{Buy: buy, Sale: sale, LastUpdate: lastUpdateStr}
	})

	// PHASE 2: CALCULATION
	// This callback runs *after* the scraping is complete.
	c.OnScraped(func(r *colly.Response) {
		if debug {
			fmt.Println("---")
			fmt.Println("Finished scraping. Found", len(exchangeRates), "currencies.")
			fmt.Println("Exchange rates last updated:", exchangeRates[strings.ToUpper(currencyFrom)].LastUpdate)
			fmt.Println("---")
		}

		// Add DKK to the map manually for easier calculations. Also this is not in the table list.
		exchangeRates["DKK"] = Rate{Buy: 100.0, Sale: 100.0, LastUpdate: "Live Rate"}

		// Get the 'from' rate. We use the Buy rate for 'from' conversions.
		fromUpper := strings.ToUpper(currencyFrom)
		fromRate, ok := exchangeRates[fromUpper]
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
			for code := range exchangeRates {
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

			// Get the 'to' rate. We use the Sale rate for 'to' conversions.
			toRate, ok := exchangeRates[toUpper]
			if !ok {
				fmt.Printf("Error: The 'to' currency '%s' not found, skipping...\n", toCurrency)
				continue
			}

			// Perform the cross-currency conversion.
			// Formula: (amount * from_rate) / to_rate
			// We use the 'Buy' rate for the 'from' currency and 'Sale' for the 'to' currency.
			finalAmount := (amount * fromRate.Buy) / toRate.Sale

			fmt.Println(toUpper, toFixed(finalAmount, 3))
		}
	})

	// Start the scrape
	if err := c.Visit(url); err != nil {
		log.Fatalf("Failed to visit URL: %v", err)
	}
}

// --- Helper Functions ---

func containsString(stringArr []string, s string) bool {
	for _, a := range stringArr {
		if s == strings.ToUpper(a) {
			return true
		}
	}
	return false
}

func normalizeNumber(old string) string {
	return strings.Replace(old, ",", ".", -1)
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
