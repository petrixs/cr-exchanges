package exchanges

import (
	"encoding/json"
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

func (k *KuCoin) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с KuCoin")

	// Сначала получим список всех контрактов
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api-futures.kucoin.com/api/v1/contracts/active", nil)
	if err != nil {
		log.Printf("Ошибка создания запроса к KuCoin: %v", err)
		return nil, err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Ошибка запроса к KuCoin: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Код ответа от KuCoin (контракты): %d", resp.StatusCode)

	var contractsResponse struct {
		Code string `json:"code"`
		Data []struct {
			Symbol              string  `json:"symbol"`
			RootSymbol          string  `json:"rootSymbol"`
			FundingFeeRate      float64 `json:"fundingFeeRate"`
			NextFundingRateTime int64   `json:"nextFundingRateTime"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&contractsResponse); err != nil {
		log.Printf("Ошибка декодирования ответа от KuCoin: %v", err)
		return nil, err
	}

	log.Printf("Получено %d контрактов от KuCoin", len(contractsResponse.Data))

	// Логируем первые несколько записей для отладки
	for i, contract := range contractsResponse.Data {
		if i < 5 {
			log.Printf("Отладка KuCoin: контракт %s, nextFundingRateTime: %d",
				contract.Symbol, contract.NextFundingRateTime)
		}
	}

	// Рассчитываем время следующего фандинга
	// KuCoin выплачивает фандинг каждые 8 часов в 00:00, 08:00, 16:00 UTC
	now := time.Now().UTC()
	fundingHours := []int{0, 8, 16}

	// Находим ближайший следующий час фандинга
	nextFundingHour := 0
	for _, hour := range fundingHours {
		if hour > now.Hour() {
			nextFundingHour = hour
			break
		}
	}

	// Если не нашли подходящий час (текущее время после 16:00), берем 00:00 следующего дня
	nextDay := 0
	if nextFundingHour == 0 && now.Hour() >= fundingHours[len(fundingHours)-1] {
		nextDay = 1
	}

	nextFundingTime := time.Date(
		now.Year(), now.Month(), now.Day()+nextDay,
		nextFundingHour, 0, 0, 0, time.UTC,
	)

	log.Printf("Текущее время UTC: %s", now.Format(time.RFC3339))
	log.Printf("Следующее время фандинга KuCoin: %s", nextFundingTime.Format(time.RFC3339))

	location := GetLocationFromEnv()
	nextFunding := nextFundingTime.In(location).Format(time.RFC3339)

	result := make([]FundingRate, 0)
	for _, contract := range contractsResponse.Data {
		fundingRate := contract.FundingFeeRate

		// Преобразуем в десятичную дробь если это процент
		if fundingRate > 1 || fundingRate < -1 {
			fundingRate = fundingRate / 100
		}

		// Используем рассчитанное фиксированное время фандинга для всех контрактов
		result = append(result, FundingRate{
			Symbol:      contract.Symbol,
			Rate:        fundingRate,
			NextFunding: nextFunding,
		})
	}

	log.Printf("Получено %d ставок фандинга с KuCoin", len(result))
	return result, nil
}
