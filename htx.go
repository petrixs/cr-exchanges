package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type HTX struct{}

func NewHTX() *HTX {
	log.Println("Инициализация HTX")
	return &HTX{}
}

func (h *HTX) GetName() string {
	return "HTX"
}

func (h *HTX) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с HTX")
	resp, err := http.Get("https://api.hbdm.com/linear-swap-api/v1/swap_batch_funding_rate")
	if err != nil {
		log.Printf("Ошибка запроса к HTX: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Status string `json:"status"`
		Data   []struct {
			Symbol        string           `json:"contract_code"`
			FundingRate   string           `json:"funding_rate"`
			NextFundingAt *json.RawMessage `json:"next_funding_time"`
			FundingTime   json.RawMessage  `json:"funding_time"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Ошибка декодирования ответа от HTX: %v", err)
		return nil, err
	}

	result := make([]FundingRate, 0)
	for _, rate := range response.Data {
		if rate.FundingRate == "" {
			continue
		}
		fundingRate := 0.0
		if _, err := fmt.Sscanf(rate.FundingRate, "%f", &fundingRate); err != nil {
			log.Printf("Ошибка конвертации ставки %s: %v", rate.FundingRate, err)
			continue
		}

		var fundingTimeMs int64
		if rate.FundingTime != nil {
			var fundingTimeStr string
			if err := json.Unmarshal(rate.FundingTime, &fundingTimeStr); err == nil {
				fundingTimeMs, _ = strconv.ParseInt(fundingTimeStr, 10, 64)
			} else {
				json.Unmarshal(rate.FundingTime, &fundingTimeMs)
			}
		}

		var nextFundingMs int64
		if rate.NextFundingAt != nil {
			var nextFundingStr string
			if err := json.Unmarshal(*rate.NextFundingAt, &nextFundingStr); err == nil {
				nextFundingMs, _ = strconv.ParseInt(nextFundingStr, 10, 64)
			} else {
				json.Unmarshal(*rate.NextFundingAt, &nextFundingMs)
			}
		}

		var nextFundingTime int64
		if nextFundingMs > 0 {
			nextFundingTime = nextFundingMs
		} else if fundingTimeMs > 0 {
			nextFundingTime = fundingTimeMs
		} else {
			nextFundingTime = 0
		}

		location := GetLocationFromEnv()
		var nextFunding string
		if nextFundingTime > 0 {
			nextFunding = time.UnixMilli(nextFundingTime).In(location).Format(time.RFC3339)
		}

		result = append(result, FundingRate{
			Symbol:      rate.Symbol,
			Rate:        fundingRate,
			NextFunding: nextFunding,
		})
	}

	log.Printf("Получено %d ставок фандинга с HTX", len(result))
	return result, nil
}
