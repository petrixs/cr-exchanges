package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type BingX struct{}

func NewBingX() *BingX {
	log.Println("Инициализация BingX")
	return &BingX{}
}

func (b *BingX) GetName() string {
	return "BingX"
}

// Структура для 24h статистики BingX
type bingXTickerResponse struct {
	Code int `json:"code"`
	Data []struct {
		Symbol      string  `json:"symbol"`
		Volume      float64 `json:"volume"`
		QuoteVolume float64 `json:"quoteVolume"`
	} `json:"data"`
}

// Получаем объемы для всех пар
func (b *BingX) getVolumes() (map[string]float64, map[string]float64, error) {
	resp, err := http.Get("https://open-api.bingx.com/openApi/swap/v2/ticker/24hr")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var tickerData bingXTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerData); err != nil {
		return nil, nil, err
	}

	if tickerData.Code != 0 {
		return nil, nil, fmt.Errorf("BingX API error: %d", tickerData.Code)
	}

	volumes := make(map[string]float64)
	volumesUSDT := make(map[string]float64)

	for _, item := range tickerData.Data {
		volumes[item.Symbol] = item.Volume
		volumesUSDT[item.Symbol] = item.QuoteVolume
	}

	return volumes, volumesUSDT, nil
}

func (b *BingX) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с BingX")

	// Получаем объемы
	volumes, volumesUSDT, err := b.getVolumes()
	if err != nil {
		log.Printf("Ошибка получения объемов BingX: %v", err)
		// Продолжаем работу без объемов
		volumes = make(map[string]float64)
		volumesUSDT = make(map[string]float64)
	}

	// Получаем фандинг ставки
	resp, err := http.Get("https://open-api.bingx.com/openApi/swap/v2/quote/fundingRate")
	if err != nil {
		log.Printf("Ошибка запроса к BingX: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Code int `json:"code"`
		Data []struct {
			Symbol          string  `json:"symbol"`
			FundingRate     float64 `json:"fundingRate"`
			FundingTime     int64   `json:"fundingTime"`
			NextFundingTime int64   `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Ошибка декодирования ответа BingX: %v", err)
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("BingX API ошибка: %d", response.Code)
	}

	var result []FundingRate
	for _, rate := range response.Data {
		var nextFunding string
		if rate.NextFundingTime > 0 {
			nextFunding = time.Unix(rate.NextFundingTime/1000, 0).Format(time.RFC3339)
		} else {
			nextFunding = time.Now().Add(time.Hour).Format(time.RFC3339)
		}

		// Получаем объемы для данного символа
		volume24h := volumes[rate.Symbol]
		volumeUSDT24h := volumesUSDT[rate.Symbol]

		result = append(result, FundingRate{
			Symbol:        rate.Symbol,
			Rate:          rate.FundingRate,
			NextFunding:   nextFunding,
			Volume24h:     volume24h,
			VolumeUSDT24h: volumeUSDT24h,
		})
	}

	log.Printf("Получено %d ставок фандинга с BingX", len(result))
	return result, nil
}
