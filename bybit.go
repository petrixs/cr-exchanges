package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Bybit struct{}

func NewBybit() *Bybit {
	log.Println("Инициализация Bybit")
	return &Bybit{}
}

func (b *Bybit) GetName() string {
	return "Bybit"
}

func (b *Bybit) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с Bybit")
	resp, err := http.Get("https://api.bybit.com/v5/market/tickers?category=linear")
	if err != nil {
		log.Printf("Ошибка запроса к Bybit: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Result struct {
			List []struct {
				Symbol        string `json:"symbol"`
				FundingRate   string `json:"fundingRate"`
				NextFundingAt string `json:"nextFundingTime"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Ошибка декодирования ответа от Bybit: %v", err)
		return nil, err
	}

	result := make([]FundingRate, 0)
	for _, rate := range response.Result.List {
		if rate.FundingRate == "" {
			continue
		}
		fundingRate := 0.0
		if _, err := fmt.Sscanf(rate.FundingRate, "%f", &fundingRate); err != nil {
			log.Printf("Ошибка конвертации ставки %s: %v", rate.FundingRate, err)
			continue
		}

		// Конвертируем timestamp в дату
		location := GetLocationFromEnv()
		nextFundingTime := rate.NextFundingAt
		if nextFundingTimestamp, err := strconv.ParseInt(rate.NextFundingAt, 10, 64); err == nil {
			nextFundingTime = time.Unix(nextFundingTimestamp/1000, 0).In(location).Format(time.RFC3339)
		}

		result = append(result, FundingRate{
			Symbol:      rate.Symbol,
			Rate:        fundingRate,
			NextFunding: nextFundingTime,
		})
	}

	log.Printf("Получено %d ставок фандинга с Bybit", len(result))
	return result, nil
}
