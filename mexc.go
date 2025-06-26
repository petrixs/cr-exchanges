package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type MEXC struct{}

func NewMEXC() *MEXC {
	log.Println("Инициализация MEXC")
	return &MEXC{}
}

func (m *MEXC) GetName() string {
	return "MEXC"
}

func (m *MEXC) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с MEXC")

	// Получаем тикеры для всех контрактов
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	tickerResp, err := client.Get("https://contract.mexc.com/api/v1/contract/ticker")
	if err != nil {
		log.Printf("Ошибка запроса тикеров к MEXC: %v", err)
		return nil, err
	}
	defer tickerResp.Body.Close()

	var tickerData struct {
		Success bool `json:"success"`
		Data    []struct {
			Symbol      string  `json:"symbol"`
			Volume24    float64 `json:"volume24"`
			Amount24    float64 `json:"amount24"`
			FundingRate float64 `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.NewDecoder(tickerResp.Body).Decode(&tickerData); err != nil {
		log.Printf("Ошибка декодирования тикеров от MEXC: %v", err)
		return nil, err
	}

	if !tickerData.Success {
		log.Printf("MEXC API вернул ошибку")
		return nil, fmt.Errorf("MEXC API error")
	}

	result := make([]FundingRate, 0)
	location := GetLocationFromEnv()

	// Берем только топ-20 символов по объему для быстрой работы
	maxSymbols := 20
	count := 0

	for _, ticker := range tickerData.Data {
		if count >= maxSymbols {
			break
		}

		// Пропускаем символы с нулевым объемом
		if ticker.Amount24 <= 1000 { // минимальный объем $1000
			continue
		}

		// Получаем детальную информацию о фандинге
		fundingURL := "https://contract.mexc.com/api/v1/contract/funding_rate/" + ticker.Symbol

		fundingResp, err := client.Get(fundingURL)
		if err != nil {
			log.Printf("Ошибка запроса фандинга для %s к MEXC: %v", ticker.Symbol, err)
			continue
		}

		var fundingData struct {
			Success bool `json:"success"`
			Data    struct {
				Symbol         string  `json:"symbol"`
				FundingRate    float64 `json:"fundingRate"`
				NextSettleTime int64   `json:"nextSettleTime"`
			} `json:"data"`
		}

		if err := json.NewDecoder(fundingResp.Body).Decode(&fundingData); err != nil {
			log.Printf("Ошибка декодирования фандинга для %s от MEXC: %v", ticker.Symbol, err)
			fundingResp.Body.Close()
			continue
		}
		fundingResp.Body.Close()

		if !fundingData.Success {
			log.Printf("MEXC API вернул ошибку для символа %s", ticker.Symbol)
			continue
		}

		result = append(result, FundingRate{
			Symbol:        fundingData.Data.Symbol,
			Rate:          fundingData.Data.FundingRate,
			NextFunding:   time.Unix(fundingData.Data.NextSettleTime/1000, 0).In(location).Format(time.RFC3339),
			Volume24h:     ticker.Volume24,
			VolumeUSDT24h: ticker.Amount24,
		})

		count++

		// Добавляем задержку между запросами
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Получено %d ставок фандинга с объемами с MEXC", len(result))
	return result, nil
}
