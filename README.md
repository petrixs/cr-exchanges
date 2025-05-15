# cr-exchanges

Модуль для получения ставок фандинга с различных криптовалютных бирж.

## Возможности
- Унифицированный интерфейс Exchange для интеграции с биржами
- Реализации для Binance, Bybit, HTX, OKX, Gate.io, KuCoin, BingX
- Лёгкое расширение: добавляйте новые биржи через реализацию интерфейса

## Пример использования

```go
import "github.com/petrixs/cr-exchanges"

binance := exchanges.NewBinance()
rates, err := binance.GetFundingRates()
if err != nil {
    // обработка ошибки
}
for _, rate := range rates {
    fmt.Println(rate.Symbol, rate.Rate)
}
```

## Интерфейс

```go
type Exchange interface {
    GetName() string
    GetFundingRates() ([]FundingRate, error)
}
```

## Поддерживаемые биржи
- Binance
- Bybit
- HTX
- OKX (API-ключи)
- Gate.io
- KuCoin
- BingX (API-ключи)

## Установка

```sh
go get github.com/petrixs/cr-exchanges
```

## Лицензия
MIT 