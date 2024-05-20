package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
	"github.com/spf13/cobra"
)

// Build A version string that can be set with
//
//	-ldflags "-X main.Build=SOMEVERSION"
//
// at compile-time.
var Build string

// List of default currency
var currencyList = []string{"EUR", "SEK", "NOK", "GBP", "USD"}

const (
	appName string = "currency converter"
)

var (
	currencyFrom string
	currencyTo   []string
	amount       float64
	debug        bool

	rootCmd = &cobra.Command{
		Use:     appName,
		Short:   "Simple fetch and convert currency tool",
		Run:     root,
		Version: Build,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&currencyFrom, "from", "f", "DKK", "Choose currency to convert from")
	rootCmd.PersistentFlags().StringSliceVarP(&currencyTo, "to", "t", currencyList, "Choose currency to convert to")
	rootCmd.PersistentFlags().Float64VarP(&amount, "amount", "a", 1, "Set amount to convert")
	rootCmd.PersistentFlags().BoolVarP(&debug, "verbose", "v", false, "Enable verbose mode")
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error:", err.Error())
	}
}

func root(_ *cobra.Command, _ []string) {
	url := "https://www.nykredit.dk/dit-liv/valutakurser/noteringskurser/"

	fmt.Println(strings.ToUpper(currencyFrom), amount)

	c := colly.NewCollector()

	c.OnHTML("td", func(e *colly.HTMLElement) {
		e.Request.Visit(e.Attr("table"))
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	c.OnHTML("table > tbody", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {

			currency := el.ChildText("td:nth-child(3)")

			sale, err := strconv.ParseFloat(normalizeNumber(el.ChildText("td:nth-child(4)")), 64)
			if err != nil {
				fmt.Println("Error parsing sale curr:", err)
			}

			buy, err := strconv.ParseFloat(normalizeNumber(el.ChildText("td:nth-child(5)")), 64)
			if err != nil {
				fmt.Println("Error parsing sale curr:", err)
			}

			lastUpdate := el.ChildText("td:nth-child(7)")

			// Convert fra DKK to any
			if strings.ToUpper(currencyFrom) == "DKK" {
				if containsString(currencyTo, currency) {
					// Output
					if debug {
						fmt.Println("Sale exchange:", sale/100)
						fmt.Println("Buy exchange:", buy/100)
						fmt.Println("Exchange rates last updated:", lastUpdate)
					}
					fmt.Println(currency, toFixed(amount/(sale/100), 3))
				}
				// Convert from any to DKK
			} else {
				if strings.ToUpper(currencyFrom) == strings.ToUpper(currency) {
					// Output
					if debug {
						fmt.Println("Sale exchange:", sale/100)
						fmt.Println("Buy exchange:", buy/100)
						fmt.Println("Exchange rates last updated:", lastUpdate)
					}

					fmt.Println("DKK", toFixed(amount*(sale/100), 3))
				}
			}
		})
	})
	c.Visit(url)
}

func containsString(stringArr []string, s string) bool {
	for _, a := range stringArr {
		if s == strings.ToUpper(a) {
			return true
		}
	}
	return false
}

func normalizeNumber(old string) string {
	s := strings.Replace(old, ",", ".", -1)
	return s
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
