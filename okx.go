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

	// Получаем объемы
	volumes, volumesUSDT, err := o.getVolumes()
	if err != nil {
		log.Printf("Ошибка получения объемов OKX: %v", err)
		// Продолжаем работу без объемов
		volumes = make(map[string]float64)
		volumesUSDT = make(map[string]float64)
	}

	// Получаем список всех инструментов
	resp, err := http.Get("https://www.okx.com/api/v5/public/instruments?instType=SWAP")
	if err != nil {
		log.Printf("Ошибка запроса инструментов OKX: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var instrumentsResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstId string `json:"instId"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&instrumentsResponse); err != nil {
		log.Printf("Ошибка декодирования инструментов OKX: %v", err)
		return nil, err
	}

	if instrumentsResponse.Code != "0" {
		return nil, fmt.Errorf("OKX API ошибка: %s - %s", instrumentsResponse.Code, instrumentsResponse.Msg)
	}

	log.Printf("Получено %d инструментов OKX", len(instrumentsResponse.Data))

	var result []FundingRate
	var nonZeroRates []string

	// Обрабатываем только первые 50 инструментов для быстрого тестирования
	maxInstruments := 50
	if len(instrumentsResponse.Data) > maxInstruments {
		instrumentsResponse.Data = instrumentsResponse.Data[:maxInstruments]
	}

	processed := 0
	for _, instrument := range instrumentsResponse.Data {
		instId := instrument.InstId

		// Получаем объемы для данного инструмента
		volume24h := volumes[instId]
		volumeUSDT24h := volumesUSDT[instId]

		fundingRate, err := o.getFundingRateForInstrument(instId, volume24h, volumeUSDT24h)
		if err != nil {
			log.Printf("Ошибка получения фандинг ставки для %s: %v", instId, err)
			continue
		}

		if fundingRate.Rate != 0 {
			nonZeroRates = append(nonZeroRates, fmt.Sprintf("%s: %f", instId, fundingRate.Rate))
		}

		result = append(result, fundingRate)
		processed++

		// Уменьшаем задержку
		time.Sleep(50 * time.Millisecond)

		// Логируем прогресс каждые 10 инструментов
		if processed%10 == 0 {
			log.Printf("Обработано %d/%d инструментов OKX", processed, len(instrumentsResponse.Data))
		}
	}

	log.Printf("OKX ненулевые ставки фандинга (%d): %v", len(nonZeroRates), nonZeroRates)
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
func (o *OKX) getFundingRateForInstrument(instId string, volume24h, volumeUSDT24h float64) (FundingRate, error) {
	// Получаем информацию о фандинг ставке для конкретного инструмента
	url := fmt.Sprintf("https://www.okx.com/api/v5/public/funding-rate?instId=%s", instId)

	resp, err := http.Get(url)
	if err != nil {
		return FundingRate{}, err
	}
	defer resp.Body.Close()

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstId          string `json:"instId"`
			FundingRate     string `json:"fundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return FundingRate{}, err
	}

	if response.Code != "0" {
		return FundingRate{}, fmt.Errorf("OKX API ошибка для %s: %s - %s", instId, response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return FundingRate{}, fmt.Errorf("нет данных о фандинг ставке для %s", instId)
	}

	data := response.Data[0]

	// Парсим ставку фандинга
	fundingRate, err := strconv.ParseFloat(data.FundingRate, 64)
	if err != nil {
		return FundingRate{}, fmt.Errorf("ошибка парсинга ставки фандинга %s: %v", data.FundingRate, err)
	}

	// Парсим время следующего фандинга
	nextFundingTimeMs, err := strconv.ParseInt(data.NextFundingTime, 10, 64)
	if err != nil {
		return FundingRate{}, fmt.Errorf("ошибка парсинга времени фандинга %s: %v", data.NextFundingTime, err)
	}

	nextFundingTime := time.Unix(nextFundingTimeMs/1000, 0).Format(time.RFC3339)

	return FundingRate{
		Symbol:        instId,
		Rate:          fundingRate,
		NextFunding:   nextFundingTime,
		Volume24h:     volume24h,
		VolumeUSDT24h: volumeUSDT24h,
	}, nil
}

// Структура для 24h статистики OKX
type okxTickerResponse struct {
	Code string `json:"code"`
	Data []struct {
		InstId    string `json:"instId"`
		Vol24h    string `json:"vol24h"`
		VolCcy24h string `json:"volCcy24h"`
	} `json:"data"`
}

// Получаем объемы для всех пар
func (o *OKX) getVolumes() (map[string]float64, map[string]float64, error) {
	resp, err := http.Get("https://www.okx.com/api/v5/market/tickers?instType=SWAP")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var tickerData okxTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerData); err != nil {
		return nil, nil, err
	}

	if tickerData.Code != "0" {
		return nil, nil, fmt.Errorf("OKX API error: %s", tickerData.Code)
	}

	volumes := make(map[string]float64)
	volumesUSDT := make(map[string]float64)

	for _, item := range tickerData.Data {
		vol24h, _ := strconv.ParseFloat(item.Vol24h, 64)
		volCcy24h, _ := strconv.ParseFloat(item.VolCcy24h, 64)

		volumes[item.InstId] = vol24h
		volumesUSDT[item.InstId] = volCcy24h
	}

	return volumes, volumesUSDT, nil
}
