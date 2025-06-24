package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Gate struct{}

func NewGate() *Gate {
	log.Println("Инициализация Gate.io")
	return &Gate{}
}

func (g *Gate) GetName() string {
	return "Gate.io"
}

func (g *Gate) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с Gate.io")
	resp, err := http.Get("https://api.gateio.ws/api/v4/futures/usdt/contracts")
	if err != nil {
		log.Printf("Ошибка запроса к Gate.io: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var contracts []struct {
		Name        string  `json:"name"`
		FundingRate string  `json:"funding_rate"`
		FundingTime int64   `json:"funding_next_apply"`
		TradeSize   float64 `json:"trade_size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&contracts); err != nil {
		log.Printf("Ошибка декодирования ответа от Gate.io: %v", err)
		return nil, err
	}

	result := make([]FundingRate, 0)
	for _, contract := range contracts {
		if contract.FundingRate == "" {
			continue
		}
		fundingRate := 0.0
		if _, err := fmt.Sscanf(contract.FundingRate, "%f", &fundingRate); err != nil {
			log.Printf("Ошибка конвертации ставки %s: %v", contract.FundingRate, err)
			continue
		}

		// Форматируем время
		location := GetLocationFromEnv()
		nextFunding := time.Unix(contract.FundingTime, 0).In(location).Format(time.RFC3339)

		result = append(result, FundingRate{
			Symbol:        contract.Name,
			Rate:          fundingRate,
			NextFunding:   nextFunding,
			Volume24h:     contract.TradeSize,
			VolumeUSDT24h: 0,
		})
	}

	log.Printf("Получено %d ставок фандинга с Gate.io", len(result))
	return result, nil
}
