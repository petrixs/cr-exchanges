package exchanges

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type BingX struct{}

func NewBingX() *BingX {
	return &BingX{}
}

func (b *BingX) GetName() string {
	return "BingX"
}

func (b *BingX) GetFundingRates() ([]FundingRate, error) {
	endpoint := "https://open-api.bingx.com/openApi/swap/v2/quote/premiumIndex"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к BingX: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа BingX: %v", err)
	}

	var response struct {
		Code int `json:"code"`
		Data []struct {
			Symbol          string `json:"symbol"`
			LastFundingRate string `json:"lastFundingRate"`
			NextFundingTime int64  `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа BingX: %v", err)
	}

	location := GetLocationFromEnv()
	result := make([]FundingRate, 0, len(response.Data))
	for _, rate := range response.Data {
		if rate.LastFundingRate == "" {
			continue
		}
		fundingRate := 0.0
		if _, err := fmt.Sscanf(rate.LastFundingRate, "%f", &fundingRate); err != nil {
			continue
		}
		nextFunding := "Неизвестно"
		if rate.NextFundingTime > 0 {
			t := time.Unix(rate.NextFundingTime/1000, 0).In(location)
			nextFunding = t.Format(time.RFC3339)
		}
		result = append(result, FundingRate{
			Symbol:      rate.Symbol,
			Rate:        fundingRate,
			NextFunding: nextFunding,
		})
	}
	return result, nil
}
