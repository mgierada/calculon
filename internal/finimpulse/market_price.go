package finimpulse

import (
	"context"
	"net/url"
	"time"
)

// MarketPrice is the current price of a symbol with the session it belongs
// to. Pointer fields are null when the API does not know them, e.g. pre- and
// post-market prices outside those sessions or market cap for funds.
type MarketPrice struct {
	Symbol                          string     `json:"symbol"`
	Name                            string     `json:"name"`
	QuoteType                       string     `json:"quote_type"`
	Currency                        string     `json:"currency"`
	RegularMarketVolume             *int64     `json:"regular_market_volume"`
	MarketCap                       *int64     `json:"market_cap"`
	USDRate                         *float64   `json:"usd_rate"`
	MarketState                     string     `json:"market_state"`
	RegularMarketOpen               *float64   `json:"regular_market_open"`
	RegularMarketPreviousClose      *float64   `json:"regular_market_previous_close"`
	CurrentPrice                    *float64   `json:"current_price"`
	CurrentPriceUSD                 *float64   `json:"current_price_usd"`
	CurrentPriceChange              *float64   `json:"current_price_change"`
	CurrentPriceChangePercent       *float64   `json:"current_price_change_percent"`
	CurrentPriceUpdateTime          *time.Time `json:"current_price_update_time"`
	RegularMarketPrice              *float64   `json:"regular_market_price"`
	RegularMarketPriceChange        *float64   `json:"regular_market_price_change"`
	RegularMarketPriceChangePercent *float64   `json:"regular_market_price_change_percent"`
	RegularMarketTime               *time.Time `json:"regular_market_time"`
	PreMarketPrice                  *float64   `json:"pre_market_price"`
	PreMarketPriceChange            *float64   `json:"pre_market_price_change"`
	PreMarketPriceChangePercent     *float64   `json:"pre_market_price_change_percent"`
	PreMarketTime                   *time.Time `json:"pre_market_time"`
	PostMarketPrice                 *float64   `json:"post_market_price"`
	PostMarketPriceChange           *float64   `json:"post_market_price_change"`
	PostMarketPriceChangePercent    *float64   `json:"post_market_price_change_percent"`
	PostMarketTime                  *time.Time `json:"post_market_time"`
}

// MarketPrice fetches the current price of a symbol in Finimpulse notation,
// e.g. XTB.WA.
func (c *Client) MarketPrice(ctx context.Context, symbol string) (Response[MarketPrice], error) {
	return get[MarketPrice](ctx, c, "/v1/market-price/"+url.PathEscape(symbol))
}
