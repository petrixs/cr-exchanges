package exchanges

import (
	"log"
	"os"
	"sync"
	"time"
)

type Exchange interface {
	GetName() string
	GetFundingRates() ([]FundingRate, error)
}

type FundingRate struct {
	Symbol        string
	Rate          float64
	NextFunding   string
	Volume24h     float64 // Объем за 24 часа в базовой валюте
	VolumeUSDT24h float64 // Объем за 24 часа в USDT
}

// RatesCache хранит кэшированные ставки фандинга
type RatesCache struct {
	Rates      map[string][]FundingRate // ключ - имя биржи
	Mu         sync.RWMutex
	LastUpdate time.Time
}

var (
	globalCache = &RatesCache{
		Rates: make(map[string][]FundingRate),
	}
)

// UpdateRates обновляет ставки в кэше для указанной биржи
func (c *RatesCache) UpdateRates(exchange Exchange) error {
	rates, err := exchange.GetFundingRates()
	if err != nil {
		return err
	}

	c.Mu.Lock()
	c.Rates[exchange.GetName()] = rates
	c.LastUpdate = time.Now()
	c.Mu.Unlock()

	return nil
}

// GetRates возвращает кэшированные ставки для указанной биржи
func (c *RatesCache) GetRates(exchangeName string) []FundingRate {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	return c.Rates[exchangeName]
}

// GetAllRates возвращает все кэшированные ставки
func (c *RatesCache) GetAllRates() map[string][]FundingRate {
	c.Mu.RLock()
	defer c.Mu.RUnlock()

	// Создаем копию карты, чтобы избежать гонки данных
	result := make(map[string][]FundingRate, len(c.Rates))
	for k, v := range c.Rates {
		rates := make([]FundingRate, len(v))
		copy(rates, v)
		result[k] = rates
	}
	return result
}

// GetLastUpdate возвращает время последнего обновления кэша
func (c *RatesCache) GetLastUpdate() time.Time {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	return c.LastUpdate
}

// GetGlobalCache возвращает глобальный кэш ставок
func GetGlobalCache() *RatesCache {
	return globalCache
}

// GetLocationFromEnv возвращает *time.Location из переменной окружения TIMEZONE, либо Local, если не задана или ошибка
func GetLocationFromEnv() *time.Location {
	tz := os.Getenv("TIMEZONE")
	if tz == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("Ошибка загрузки таймзоны %s: %v, используется Local", tz, err)
		return time.Local
	}
	return loc
}
