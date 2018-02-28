package goex

type RealTimeAPI interface {
	GetTickerChan(currency CurrencyPair) (chan *Ticker, chan struct{}, error)
	GetDepthChan(currency CurrencyPair) (chan *Depth, chan struct{}, error)
	GetKlineRecordsChan(currency CurrencyPair, period int) (chan []Kline, chan struct{}, error)
	GetTradesChan(currencyPair CurrencyPair) (chan []Trade, chan struct{}, error)
}
