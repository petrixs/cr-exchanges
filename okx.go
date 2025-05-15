package exchanges

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

type OKX struct {
	ApiKey     string
	SecretKey  string
	Passphrase string
}

func NewOKX() *OKX {
	log.Println("Инициализация OKX")

	apiKey := os.Getenv("OKX_API_KEY")
	secretKey := os.Getenv("OKX_SECRET_KEY")
	passphrase := os.Getenv("OKX_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		log.Println("Предупреждение: Ключи API для OKX не настроены, используются тестовые данные")
		return &OKX{}
	}

	log.Printf("OKX API ключи настроены: ApiKey: %s... SecretKey: %s... Passphrase: %s...",
		apiKey[:5]+"...",
		secretKey[:5]+"...",
		passphrase[:2]+"...")

	return &OKX{
		ApiKey:     apiKey,
		SecretKey:  secretKey,
		Passphrase: passphrase,
	}
}

func (o *OKX) GetName() string {
	return "OKX"
}

func (o *OKX) signRequest(timestamp, method, requestPath string, body string) string {
	message := timestamp + method + requestPath + body

	h := hmac.New(sha256.New, []byte(o.SecretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (o *OKX) GetFundingRates() ([]FundingRate, error) {
	log.Println("Запрос ставок фандинга с OKX")

	if o.ApiKey == "" || o.SecretKey == "" || o.Passphrase == "" {
		// Если ключи API не настроены, возвращаем ошибку
		log.Println("Ошибка: Ключи API для OKX не настроены")
		return nil, fmt.Errorf("OKX API ключи не настроены")
	}

	// Шаг 1: Сначала получим список инструментов
	instruments, err := o.getInstruments()
	if err != nil {
		return nil, err
	}

	log.Printf("Получено %d инструментов от OKX", len(instruments))

	// Шаг 2: Получаем ставки фандинга для каждого инструмента
	result := make([]FundingRate, 0, len(instruments))

	// Подробное логирование ставок
	var allRates []string
	var highRates []string

	// Для соблюдения ограничения API: 20 запросов в 2 секунды (10 запросов в секунду)
	rateLimiter := time.NewTicker(100 * time.Millisecond) // 100 мс между запросами = 10 запросов в секунду
	defer rateLimiter.Stop()

	log.Printf("Начинаем получение ставок фандинга для %d инструментов...", len(instruments))

	for i, instId := range instruments {
		// Ожидаем тикера для соблюдения ограничения API
		<-rateLimiter.C

		rate, err := o.getFundingRate(instId)
		if err != nil {
			log.Printf("Ошибка получения ставки для %s: %v", instId, err)
			continue
		}

		if rate.Rate != 0 {
			result = append(result, rate)

			// Сохраняем для логирования
			rateStr := fmt.Sprintf("%s: %.6f", rate.Symbol, rate.Rate)
			allRates = append(allRates, rateStr)

			// Высокая ставка (по абсолютному значению больше 0.00001 или 0.001%)
			if rate.Rate > 0.00001 || rate.Rate < -0.00001 {
				highRates = append(highRates, rateStr)
			}
		}

		// Логируем прогресс каждые 10 инструментов
		if (i+1)%10 == 0 {
			log.Printf("Обработано %d/%d инструментов OKX", i+1, len(instruments))
		}
	}

	// Сортируем и выводим ставки для отладки
	sort.Strings(highRates)
	sort.Strings(allRates)

	// log.Printf("OKX все ставки фандинга (%d): %v", len(allRates), allRates)

	log.Printf("OKX ненулевые ставки фандинга (%d): %v", len(highRates), highRates)
	log.Printf("Получено %d ставок фандинга с OKX", len(result))

	return result, nil
}

// getInstruments получает список инструментов с OKX
func (o *OKX) getInstruments() ([]string, error) {
	endpoint := "/api/v5/public/instruments"
	queryParams := "?instType=SWAP"
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	signature := o.signRequest(timestamp, "GET", endpoint+queryParams, "")

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.okx.com"+endpoint+queryParams, nil)
	if err != nil {
		log.Printf("Ошибка создания запроса к OKX (instruments): %v", err)
		return nil, err
	}

	// Добавляем заголовки для аутентификации
	req.Header.Add("OK-ACCESS-KEY", o.ApiKey)
	req.Header.Add("OK-ACCESS-SIGN", signature)
	req.Header.Add("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Add("OK-ACCESS-PASSPHRASE", o.Passphrase)
	req.Header.Add("Content-Type", "application/json")

	// Логирование для отладки
	log.Printf("OKX запрос списка инструментов: %s", req.URL.String())

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Ошибка запроса инструментов к OKX: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Код ответа от OKX (instruments): %d", resp.StatusCode)

	// Чтение и логирование ответа
	respBody, _ := ioutil.ReadAll(resp.Body)

	// Для экономии места в логах выводим только первые 300 символов ответа
	if len(respBody) > 300 {
		log.Printf("OKX ответ (первые 300 символов): %s...", string(respBody)[:300])
	} else {
		log.Printf("OKX ответ: %s", string(respBody))
	}

	// Проверяем корректность ответа
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OKX API вернул код ошибки %d: %s", resp.StatusCode, string(respBody))
	}

	type Instrument struct {
		InstID string `json:"instId"`
	}

	var response struct {
		Code string       `json:"code"`
		Data []Instrument `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("ошибка декодирования списка инструментов от OKX: %v", err)
	}

	// Извлекаем идентификаторы инструментов
	var instruments []string
	for _, instrument := range response.Data {
		instruments = append(instruments, instrument.InstID)
	}

	return instruments, nil
}

// getFundingRate получает ставку фандинга для конкретного инструмента
func (o *OKX) getFundingRate(instId string) (FundingRate, error) {
	endpoint := "/api/v5/public/funding-rate"
	queryParams := fmt.Sprintf("?instId=%s", instId)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	signature := o.signRequest(timestamp, "GET", endpoint+queryParams, "")

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.okx.com"+endpoint+queryParams, nil)
	if err != nil {
		return FundingRate{}, fmt.Errorf("ошибка создания запроса ставки фандинга: %v", err)
	}

	// Добавляем заголовки для аутентификации
	req.Header.Add("OK-ACCESS-KEY", o.ApiKey)
	req.Header.Add("OK-ACCESS-SIGN", signature)
	req.Header.Add("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Add("OK-ACCESS-PASSPHRASE", o.Passphrase)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return FundingRate{}, fmt.Errorf("ошибка запроса ставки фандинга: %v", err)
	}
	defer resp.Body.Close()

	// Чтение ответа
	respBody, _ := ioutil.ReadAll(resp.Body)

	// Проверяем корректность ответа
	if resp.StatusCode != 200 {
		return FundingRate{}, fmt.Errorf("код ошибки %d для %s: %s", resp.StatusCode, instId, string(respBody))
	}

	type FundingRateData struct {
		InstID      string `json:"instId"`
		FundingRate string `json:"fundingRate"`
		FundingTime string `json:"fundingTime"`
	}

	var response struct {
		Code string            `json:"code"`
		Data []FundingRateData `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return FundingRate{}, fmt.Errorf("ошибка декодирования ставки фандинга: %v", err)
	}

	// Если данных нет или пустой массив
	if len(response.Data) == 0 {
		return FundingRate{}, fmt.Errorf("нет данных по ставке фандинга")
	}

	// Получаем данные из ответа
	data := response.Data[0] // Берем первый элемент массива
	fundingRate := 0.0

	// Конвертируем строку в число
	if data.FundingRate != "" {
		if _, err := fmt.Sscanf(data.FundingRate, "%f", &fundingRate); err != nil {
			log.Printf("Ошибка конвертации ставки %s: %v", data.FundingRate, err)
		}
	}

	// Определяем время следующей выплаты
	location := GetLocationFromEnv()
	nextFundingTime := ""
	if data.FundingTime != "" {
		if fundingTimestamp, err := strconv.ParseInt(data.FundingTime, 10, 64); err == nil {
			t := time.Unix(fundingTimestamp/1000, 0)
			nextFundingTime = t.In(location).Format(time.RFC3339)
		}
	}

	return FundingRate{
		Symbol:      instId,
		Rate:        fundingRate,
		NextFunding: nextFundingTime,
	}, nil
}
