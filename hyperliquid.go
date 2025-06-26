package exchanges

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Hyperliquid struct {
	client *http.Client
}

func NewHyperliquid() *Hyperliquid {
	return &Hyperliquid{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *Hyperliquid) GetName() string {
	return "Hyperliquid"
}

type HyperliquidInfoRequest struct {
	Type string `json:"type"`
}

type HyperliquidMetaAndAssetCtxsResponse []interface{}

type HyperliquidUniverse struct {
	Name         string `json:"name"`
	SzDecimals   int    `json:"szDecimals"`
	MaxLeverage  int    `json:"maxLeverage"`
	OnlyIsolated bool   `json:"onlyIsolated,omitempty"`
	IsDelisted   bool   `json:"isDelisted,omitempty"`
}

type HyperliquidAssetContext struct {
	DayNtlVlm    string   `json:"dayNtlVlm"`           // 24h volume in notional
	Funding      string   `json:"funding"`             // current funding rate
	MarkPx       string   `json:"markPx"`              // mark price
	MidPx        string   `json:"midPx"`               // mid price
	OpenInterest string   `json:"openInterest"`        // open interest
	OraclePx     string   `json:"oraclePx"`            // oracle price
	Premium      string   `json:"premium"`             // premium
	PrevDayPx    string   `json:"prevDayPx"`           // previous day price
	ImpactPxs    []string `json:"impactPxs,omitempty"` // impact prices [bid, ask]
}

func (h *Hyperliquid) GetFundingRates() ([]FundingRate, error) {
	requestBody := HyperliquidInfoRequest{
		Type: "metaAndAssetCtxs",
	}

	reqBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.hyperliquid.xyz/info", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	var response HyperliquidMetaAndAssetCtxsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if len(response) < 2 {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Parse metadata (first element)
	metaBytes, err := json.Marshal(response[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %v", err)
	}

	var meta struct {
		Universe []HyperliquidUniverse `json:"universe"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %v", err)
	}

	// Parse asset contexts (second element)
	assetCtxsBytes, err := json.Marshal(response[1])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal asset contexts: %v", err)
	}

	var assetCtxs []HyperliquidAssetContext
	if err := json.Unmarshal(assetCtxsBytes, &assetCtxs); err != nil {
		return nil, fmt.Errorf("failed to decode asset contexts: %v", err)
	}

	var fundingRates []FundingRate

	for i, ctx := range assetCtxs {
		if i >= len(meta.Universe) {
			break
		}

		asset := meta.Universe[i]

		// Skip delisted assets
		if asset.IsDelisted {
			continue
		}

		// Parse funding rate
		fundingRate, err := strconv.ParseFloat(ctx.Funding, 64)
		if err != nil {
			continue
		}

		// Parse 24h volume
		volume24h, err := strconv.ParseFloat(ctx.DayNtlVlm, 64)
		if err != nil {
			volume24h = 0
		}

		// Filter out low volume symbols (less than $1000)
		if volume24h < 1000 {
			continue
		}

		// Calculate next funding time (Hyperliquid funding is every hour)
		now := time.Now()
		location := GetLocationFromEnv()
		nextHour := now.Truncate(time.Hour).Add(time.Hour)
		nextFundingString := nextHour.In(location).Format(time.RFC3339)

		fundingRates = append(fundingRates, FundingRate{
			Symbol:        asset.Name,
			Rate:          fundingRate,
			NextFunding:   nextFundingString,
			Volume24h:     volume24h,
			VolumeUSDT24h: volume24h, // On Hyperliquid, volume is already in USDT/USD
		})
	}

	return fundingRates, nil
}
