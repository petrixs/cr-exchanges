package exchanges

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Binance struct{}

func NewBinance() *Binance {
	log.Println("Инициализация Binance")
	return &Binance{}
}

func (b *Binance) GetName() string {
	return "Binance"
}

func (b *Binance) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с Binance")
	resp, err := http.Get("https://fapi.binance.com/fapi/v1/premiumIndex")
	if err != nil {
		log.Printf("Ошибка запроса к Binance: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var rates []struct {
		Symbol          string  `json:"symbol"`
		LastFundingRate float64 `json:"lastFundingRate,string"`
		NextFundingTime int64   `json:"nextFundingTime"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		log.Printf("Ошибка декодирования ответа от Binance: %v", err)
		return nil, err
	}

	result := make([]FundingRate, len(rates))
	for i, rate := range rates {
		location := GetLocationFromEnv()
		result[i] = FundingRate{
			Symbol:      rate.Symbol,
			Rate:        rate.LastFundingRate,
			NextFunding: time.Unix(rate.NextFundingTime/1000, 0).In(location).Format(time.RFC3339),
		}
	}

	log.Printf("Получено %d ставок фандинга с Binance", len(result))
	return result, nil
}
