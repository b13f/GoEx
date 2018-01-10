package bittrex

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	. "github.com/nntaoli-project/GoEx"
)

type Bittrex struct {
	client *http.Client
	baseUrl,
	accessKey,
	secretKey string
}

func New(client *http.Client, accesskey, secretkey string) *Bittrex {
	return &Bittrex{client: client, accessKey: accesskey, secretKey: secretkey, baseUrl: "https://bittrex.com/api/v1.1"}
}

func (bx *Bittrex) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return bx.placeOrder(amount, price, currency, `buy`)
}
func (bx *Bittrex) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return bx.placeOrder(amount, price, currency, `sell`)
}
func (bx *Bittrex) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("not implement")
}
func (bx *Bittrex) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("not implement")
}
func (bx *Bittrex) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	uri := fmt.Sprintf("%s/market/cancel", bx.baseUrl)

	req, _ := url.Parse(uri)
	t := req.Query()

	t.Set(`uuid`, orderId)

	t.Set(`apikey`, bx.accessKey)
	t.Set(`nonce`, fmt.Sprintf("%d", time.Now().UnixNano()))
	req.RawQuery = t.Encode()

	headers := make(map[string]string)
	headers[`apisign`] = bx.getSign(req.String())

	resp, err := HttpGet2(bx.client, req.String(), headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return false, errCode
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return false, fmt.Errorf("%s", resp["message"].(string))
	}

	return true, nil
}
func (bx *Bittrex) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	panic("not implement")
}
func (bx *Bittrex) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	panic("not implement")
}
func (bx *Bittrex) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	panic("not implement")
}
func (bx *Bittrex) GetAccount() (*Account, error) {
	uri := fmt.Sprintf("%s/account/getbalances", bx.baseUrl)

	req, _ := url.Parse(uri)
	t := req.Query()
	t.Set(`apikey`, bx.accessKey)
	t.Set(`nonce`, fmt.Sprintf("%d", time.Now().UnixNano()))
	req.RawQuery = t.Encode()

	headers := make(map[string]string)
	headers[`apisign`] = bx.getSign(req.String())

	resp, err := HttpGet2(bx.client, req.String(), headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, errCode
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["message"].(string))
	}

	acc := Account{}
	acc.Exchange = bx.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	balances := resp["result"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := NewCurrency(vv["Currency"].(string), "")
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["Available"]),
			ForzenAmount: ToFloat64(vv["Pending"]),
		}
	}

	return &acc, nil
}

func (bx *Bittrex) GetTicker(currency CurrencyPair) (*Ticker, error) {
	resp, err := HttpGet(bx.client, fmt.Sprintf("%s/public/getmarketsummary?market=%s", bx.baseUrl, currency.ToSymbol2("-")))
	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, errCode
	}

	result, _ := resp["result"].([]interface{})
	if len(result) <= 0 {
		return nil, API_ERR
	}

	tickermap := result[0].(map[string]interface{})

	return &Ticker{
		Last: ToFloat64(tickermap["Last"]),
		Sell: ToFloat64(tickermap["Ask"]),
		Buy:  ToFloat64(tickermap["Bid"]),
		Low:  ToFloat64(tickermap["Low"]),
		High: ToFloat64(tickermap["High"]),
		Vol:  ToFloat64(tickermap["Volume"]),
	}, nil
}

func (bx *Bittrex) GetDepth(size int, currency CurrencyPair) (*Depth, error) {

	resp, err := HttpGet(bx.client, fmt.Sprintf("%s/public/getorderbook?market=%s&type=both", bx.baseUrl, currency.ToSymbol2("-")))
	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, errCode
	}

	result := resp["result"].(map[string]interface{})

	bids, _ := result["buy"].([]interface{})
	asks, _ := result["sell"].([]interface{})

	dep := new(Depth)

	for _, v := range bids {
		r := v.(map[string]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(r["Rate"]), ToFloat64(r["Quantity"])})
	}

	for _, v := range asks {
		r := v.(map[string]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(r["Rate"]), ToFloat64(r["Quantity"])})
	}

	sort.Sort(sort.Reverse(dep.AskList))

	return dep, nil
}

func (bx *Bittrex) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, error) {
	panic("not implement")
}

//非个人，整个交易所的交易记录
func (bx *Bittrex) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implement")
}

func (bx *Bittrex) GetExchangeName() string {
	return "bittrex.com"
}

func (bx *Bittrex) getSign(uri string) string {
	sign, _ := GetParamHmacSHA512Sign(bx.secretKey, uri)
	return sign
}

func (bx *Bittrex) placeOrder(amount, price string, pair CurrencyPair, orderSide string) (*Order, error) {
	uri := fmt.Sprintf("%s/market/%slimit", bx.baseUrl, orderSide)

	req, _ := url.Parse(uri)
	t := req.Query()

	t.Set(`market`, pair.ToSymbol(`-`))
	t.Set(`quantity`, amount)
	t.Set(`rate`, price)

	t.Set(`apikey`, bx.accessKey)
	t.Set(`nonce`, fmt.Sprintf("%d", time.Now().UnixNano()))
	req.RawQuery = t.Encode()

	headers := make(map[string]string)
	headers[`apisign`] = bx.getSign(req.String())

	resp, err := HttpGet2(bx.client, req.String(), headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, errCode
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["message"].(string))
	}

	res := resp["result"].(map[string]interface{})

	side := BUY
	if orderSide == "sell" {
		side = SELL
	}
	return &Order{
		Currency:   pair,
		OrderID2:   res[`uuid`].(string),
		Price:      ToFloat64(price),
		Amount:     ToFloat64(amount),
		DealAmount: 0,
		AvgPrice:   0,
		Side:       TradeSide(side),
		Status:     ORDER_UNFINISH,
		OrderTime:  int(time.Now().Unix())}, nil
}
