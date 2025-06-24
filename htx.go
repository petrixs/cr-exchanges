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

// Структура для объемов HTX
type htxVolumeResponse struct {
	Status string `json:"status"`
	Ticks  []struct {
		ContractCode  string `json:"contract_code"`
		Vol           string `json:"vol"`
		TradeTurnover string `json:"trade_turnover"`
	} `json:"ticks"`
}

// Получаем объемы для всех пар
func (h *HTX) getVolumes() (map[string]float64, map[string]float64, error) {
	resp, err := http.Get("https://api.hbdm.com/linear-swap-ex/market/detail/batch_merged")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var volumeData htxVolumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&volumeData); err != nil {
		return nil, nil, err
	}

	volumes := make(map[string]float64)
	volumesUSDT := make(map[string]float64)

	for _, item := range volumeData.Ticks {
		// Используем символ как есть из contract_code
		vol, _ := strconv.ParseFloat(item.Vol, 64)
		turnover, _ := strconv.ParseFloat(item.TradeTurnover, 64)
		volumes[item.ContractCode] = vol
		volumesUSDT[item.ContractCode] = turnover
	}

	return volumes, volumesUSDT, nil
}

func (h *HTX) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с HTX")

	// Получаем объемы
	volumes, volumesUSDT, err := h.getVolumes()
	if err != nil {
		log.Printf("Ошибка получения объемов HTX: %v", err)
		// Продолжаем работу без объемов
		volumes = make(map[string]float64)
		volumesUSDT = make(map[string]float64)
	}

	// Получаем фандинг ставки
	resp, err := http.Get("https://api.hbdm.com/linear-swap-api/v1/swap_batch_funding_rate")
	if err != nil {
		log.Printf("Ошибка запроса к HTX: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Status string `json:"status"`
		Data   []struct {
			Symbol      string  `json:"symbol"`
			FundingRate float64 `json:"funding_rate,string"`
			FundingTime string  `json:"funding_time"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Ошибка декодирования ответа HTX: %v", err)
		return nil, err
	}

	if response.Status != "ok" {
		return nil, fmt.Errorf("HTX API ошибка: %s", response.Status)
	}

	var result []FundingRate
	for _, rate := range response.Data {
		fundingTime, err := strconv.ParseInt(rate.FundingTime, 10, 64)
		if err != nil {
			continue
		}

		nextFunding := time.Unix(fundingTime/1000, 0).Format(time.RFC3339)

		// Получаем объемы для данного символа, добавляем -USDT если нужно
		symbolKey := rate.Symbol + "-USDT"
		volume24h := volumes[symbolKey]
		volumeUSDT24h := volumesUSDT[symbolKey]

		result = append(result, FundingRate{
			Symbol:        rate.Symbol,
			Rate:          rate.FundingRate,
			NextFunding:   nextFunding,
			Volume24h:     volume24h,
			VolumeUSDT24h: volumeUSDT24h,
		})
	}

	log.Printf("Получено %d ставок фандинга с HTX", len(result))
	return result, nil
}
