package portfolio

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FX converts amounts into the base currency the report totals are shown in.
// It is an interface so a live rates API can replace the static table.
type FX interface {
	// Base is the currency every converted amount is expressed in.
	Base() string
	// Rate is how many units of the base one unit of currency is worth.
	Rate(currency string) (float64, bool)
	// Describe says where the rates come from, for display next to totals.
	Describe() string
}

// StaticFX is a fixed rate table, typically read from configuration.
type StaticFX struct {
	base  string
	rates map[string]float64
}

// NewStaticFX builds a rate table. The base currency always converts at 1.
func NewStaticFX(base string, rates map[string]float64) StaticFX {
	all := map[string]float64{base: 1}
	for currency, rate := range rates {
		all[currency] = rate
	}
	return StaticFX{base: base, rates: all}
}

// ParseRates reads "USD=3.65,EUR=4.26" into a rate table.
func ParseRates(spec string) (map[string]float64, error) {
	rates := map[string]float64{}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		currency, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("fx rate %q: want CURRENCY=RATE", pair)
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || rate <= 0 {
			return nil, fmt.Errorf("fx rate %q: rate must be a positive number", pair)
		}
		rates[strings.ToUpper(strings.TrimSpace(currency))] = rate
	}
	return rates, nil
}

// Base implements FX.
func (s StaticFX) Base() string {
	return s.base
}

// Rate implements FX.
func (s StaticFX) Rate(currency string) (float64, bool) {
	rate, ok := s.rates[currency]
	return rate, ok
}

// Describe implements FX.
func (s StaticFX) Describe() string {
	pairs := make([]string, 0, len(s.rates))
	for currency, rate := range s.rates {
		if currency == s.base {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s %s=%g", currency, s.base, rate))
	}
	if len(pairs) == 0 {
		return "no FX"
	}
	sort.Strings(pairs)
	return "static FX " + strings.Join(pairs, ", ")
}
