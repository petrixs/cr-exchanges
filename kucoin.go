package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type KuCoin struct{}

func NewKuCoin() *KuCoin {
	log.Println("Инициализация KuCoin")
	return &KuCoin{}
}

func (k *KuCoin) GetName() string {
	return "KuCoin"
}

// Структура для 24h статистики KuCoin
type kuCoinStatsResponse struct {
	Code string `json:"code"`
	Data []struct {
		Symbol   string  `json:"symbol"`
		Vol      float64 `json:"vol"`
		Turnover float64 `json:"turnover"`
	} `json:"data"`
}

// Получаем объемы для всех пар
func (k *KuCoin) getVolumes() (map[string]float64, map[string]float64, error) {
	resp, err := http.Get("https://api-futures.kucoin.com/api/v1/contracts/stats")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var statsData kuCoinStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsData); err != nil {
		return nil, nil, err
	}

	if statsData.Code != "200000" {
		return nil, nil, fmt.Errorf("KuCoin API error: %s", statsData.Code)
	}

	volumes := make(map[string]float64)
	volumesUSDT := make(map[string]float64)

	for _, item := range statsData.Data {
		volumes[item.Symbol] = item.Vol
		volumesUSDT[item.Symbol] = item.Turnover
	}

	return volumes, volumesUSDT, nil
}

func (k *KuCoin) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с KuCoin")

	// Объемы теперь получаем из API контрактов напрямую

	// Получаем контракты
	contractsResp, err := http.Get("https://api-futures.kucoin.com/api/v1/contracts/active")
	if err != nil {
		log.Printf("Ошибка запроса контрактов KuCoin: %v", err)
		return nil, err
	}
	defer contractsResp.Body.Close()

	var contractsResponse struct {
		Code string `json:"code"`
		Data []struct {
			Symbol        string  `json:"symbol"`
			FundingRate   float64 `json:"fundingFeeRate"`
			VolumeOf24h   float64 `json:"volumeOf24h"`
			TurnoverOf24h float64 `json:"turnoverOf24h"`
		} `json:"data"`
	}

	if err := json.NewDecoder(contractsResp.Body).Decode(&contractsResponse); err != nil {
		log.Printf("Ошибка декодирования контрактов KuCoin: %v", err)
		return nil, err
	}

	if contractsResponse.Code != "200000" {
		return nil, fmt.Errorf("KuCoin API ошибка: %s", contractsResponse.Code)
	}

	var result []FundingRate
	for _, contract := range contractsResponse.Data {
		fundingRate := contract.FundingRate

		nextFunding := time.Now().Add(time.Hour).Format(time.RFC3339)

		// Используем объемы из API контрактов
		volume24h := contract.VolumeOf24h
		volumeUSDT24h := contract.TurnoverOf24h

		result = append(result, FundingRate{
			Symbol:        contract.Symbol,
			Rate:          fundingRate,
			NextFunding:   nextFunding,
			Volume24h:     volume24h,
			VolumeUSDT24h: volumeUSDT24h,
		})
	}

	log.Printf("Получено %d ставок фандинга с KuCoin", len(result))
	return result, nil
}
