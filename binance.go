package exchanges

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
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

	// Получаем фандинг ставки
	fundingResp, err := http.Get("https://fapi.binance.com/fapi/v1/premiumIndex")
	if err != nil {
		log.Printf("Ошибка запроса фандинга к Binance: %v", err)
		return nil, err
	}
	defer fundingResp.Body.Close()

	var fundingRates []struct {
		Symbol          string  `json:"symbol"`
		LastFundingRate float64 `json:"lastFundingRate,string"`
		NextFundingTime int64   `json:"nextFundingTime"`
	}

	if err := json.NewDecoder(fundingResp.Body).Decode(&fundingRates); err != nil {
		log.Printf("Ошибка декодирования фандинга от Binance: %v", err)
		return nil, err
	}

	// Получаем объемы торгов
	volumeResp, err := http.Get("https://fapi.binance.com/fapi/v1/ticker/24hr")
	if err != nil {
		log.Printf("Ошибка запроса объемов к Binance: %v", err)
		return nil, err
	}
	defer volumeResp.Body.Close()

	var volumeData []struct {
		Symbol      string `json:"symbol"`
		Volume      string `json:"volume"`
		QuoteVolume string `json:"quoteVolume"`
	}

	if err := json.NewDecoder(volumeResp.Body).Decode(&volumeData); err != nil {
		log.Printf("Ошибка декодирования объемов от Binance: %v", err)
		return nil, err
	}

	// Создаем карту объемов для быстрого поиска
	volumeMap := make(map[string]struct {
		Volume      float64
		QuoteVolume float64
	})

	for _, vol := range volumeData {
		volume, _ := strconv.ParseFloat(vol.Volume, 64)
		quoteVolume, _ := strconv.ParseFloat(vol.QuoteVolume, 64)
		volumeMap[vol.Symbol] = struct {
			Volume      float64
			QuoteVolume float64
		}{volume, quoteVolume}
	}

	// Объединяем данные
	result := make([]FundingRate, len(fundingRates))
	location := GetLocationFromEnv()

	for i, rate := range fundingRates {
		var volume24h, volumeUSDT24h float64
		if vol, exists := volumeMap[rate.Symbol]; exists {
			volume24h = vol.Volume
			volumeUSDT24h = vol.QuoteVolume
		}

		result[i] = FundingRate{
			Symbol:        rate.Symbol,
			Rate:          rate.LastFundingRate,
			NextFunding:   time.Unix(rate.NextFundingTime/1000, 0).In(location).Format(time.RFC3339),
			Volume24h:     volume24h,
			VolumeUSDT24h: volumeUSDT24h,
		}
	}

	log.Printf("Получено %d ставок фандинга с объемами с Binance", len(result))
	return result, nil
}
