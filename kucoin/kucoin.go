package kucoin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	. "github.com/nntaoli-project/GoEx"
)

type Kucoin struct {
	client *http.Client
	baseUrl,
	apiKey,
	secretKey string
}

func New(client *http.Client, apiKey, secretKey string) *Kucoin {
	return &Kucoin{client: client, apiKey: apiKey, secretKey: secretKey, baseUrl: "https://api.kucoin.com"}
}

func (ku *Kucoin) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return ku.placeOrder(amount, price, currency, `BUY`)
}
func (ku *Kucoin) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return ku.placeOrder(amount, price, currency, `SELL`)
}
func (ku *Kucoin) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("not implement")
}
func (ku *Kucoin) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("not implement")
}

func (ku *Kucoin) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	uri := fmt.Sprintf("%s/v1/cancel-order?symbol="+currency.ToSymbol(`-`), ku.baseUrl)

	//if type is wrong still get SUCCESS
	headers := ku.getSign(`/v1/cancel-order`, map[string]string{
		"symbol":   currency.ToSymbol(`-`),
		"orderOid": orderId,
		//TODO: type is needed
		"type": "BUY",
	})

	respBytes, err := HttpPostForm2(ku.client, uri, url.Values{
		"orderOid": []string{orderId},
		"type":     []string{"BUY"},
	}, headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return false, errCode
	}

	headers = ku.getSign(`/v1/cancel-order`, map[string]string{
		"symbol":   currency.ToSymbol(`-`),
		"orderOid": orderId,
		//TODO: type is needed
		"type": "SELL",
	})

	respBytes, err = HttpPostForm2(ku.client, uri, url.Values{
		"orderOid": []string{orderId},
		"type":     []string{"SELL"},
	}, headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return false, errCode
	}

	var resp map[string]interface{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return false, err
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return false, fmt.Errorf("%s", resp["message"].(string))
	}

	return true, nil
}

func (ku *Kucoin) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	types := []string{"BUY", "SELL"}

	var resp map[string]interface{}
	var err error

	for _, t := range types {
		uri := fmt.Sprintf("%s/v1/order/detail?orderOid=%s&symbol=%s&type=%s",
			ku.baseUrl, orderId, currency.ToSymbol(`-`), t)

		headers := ku.getSign(`/v1/order/detail`, map[string]string{
			"orderOid": orderId,
			"symbol":   currency.ToSymbol(`-`),
			"type":     t,
		})

		resp, err = HttpGet2(ku.client, uri, headers)

		if err != nil {
			errCode := HTTP_ERR_CODE
			errCode.OriginErrMsg = err.Error()
			return nil, err
		}

		if resp["data"] != nil {
			break
		}
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["message"].(string))
	}
	if resp["data"] == nil {
		return nil, fmt.Errorf("Order not exist")
	}

	o := resp["data"].(map[string]interface{})

	order := Order{
		OrderID2:   o["orderOid"].(string),
		Amount:     o["dealAmount"].(float64) + o["pendingAmount"].(float64),
		Price:      o["orderPrice"].(float64),
		AvgPrice:   o["dealPriceAverage"].(float64),
		DealAmount: o["dealAmount"].(float64),
		Currency: CurrencyPair{
			Currency{Symbol: o["coinType"].(string)},
			Currency{Symbol: o["coinTypePair"].(string)},
		},
		Fee:       o["feeTotal"].(float64),
		OrderTime: int(o["createdAt"].(float64)),
	}

	if o["dealAmount"].(float64) == 0 {
		order.Status = ORDER_UNFINISH
	}
	if o["pendingAmount"].(float64) == 0 {
		order.Status = ORDER_FINISH
	}
	if o["pendingAmount"].(float64) > 0 && o["dealAmount"].(float64) > 0 {
		order.Status = ORDER_PART_FINISH
	}

	if o["type"].(string) == "SELL" {
		order.Side = SELL
	} else if o["type"].(string) == "BUY" {
		order.Side = BUY
	}

	return &order, nil
}

func (ku *Kucoin) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	uri := fmt.Sprintf("%s/v1/order/active-map?symbol="+currency.ToSymbol(`-`), ku.baseUrl)

	headers := ku.getSign(`/v1/order/active-map`, map[string]string{
		"symbol": currency.ToSymbol(`-`),
	})

	resp, err := HttpGet2(ku.client, uri, headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, err
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["message"].(string))
	}

	datamapAll := resp["data"].(map[string]interface{})
	datamap := datamapAll["SELL"].([]interface{})
	datamap = append(datamap, datamapAll["BUY"].([]interface{})...)

	var orders []Order
	for _, v := range datamap {
		o := v.(map[string]interface{})

		order := Order{
			OrderID2:   o["oid"].(string),
			Amount:     o["dealAmount"].(float64) + o["pendingAmount"].(float64),
			Price:      o["price"].(float64),
			DealAmount: o["dealAmount"].(float64),
			OrderTime:  int(o["createdAt"].(float64)),
			Currency:   currency,
		}

		typeS := o["direction"].(string)
		switch typeS {
		case "SELL":
			order.Side = SELL
		case "BUY":
			order.Side = BUY
		}

		if o["dealAmount"].(float64) == 0 {
			order.Status = ORDER_UNFINISH
		}
		if o["pendingAmount"].(float64) > 0 && o["dealAmount"].(float64) > 0 {
			order.Status = ORDER_PART_FINISH
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func (ku *Kucoin) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	panic("not implement")
}
func (ku *Kucoin) GetAccount() (*Account, error) {
	uri := fmt.Sprintf("%s/v1/account/balance?limit=20", ku.baseUrl)

	headers := ku.getSign(`/v1/account/balance`, map[string]string{
		"limit": "20",
	})

	resp, err := HttpGet2(ku.client, uri, headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, err
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["message"].(string))
	}

	acc := Account{}
	acc.Exchange = ku.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	balances := resp["data"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})

		if ToFloat64(vv["balance"]) == 0 {
			continue
		}

		currency := NewCurrency(vv["coinType"].(string), "")
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["balance"]),
			ForzenAmount: ToFloat64(vv["freezeBalance"]),
		}
	}

	return &acc, nil
}

func (ku *Kucoin) GetTicker(currency CurrencyPair) (*Ticker, error) {
	panic("not implement")
}

func (ku *Kucoin) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	resp, err := HttpGet(ku.client, fmt.Sprintf("%s/v1/open/orders?symbol=%s&limit=%d", ku.baseUrl, currency.ToSymbol("-"), size))

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, err
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		switch m := resp["message"].(type) {
		case string:
			return nil, fmt.Errorf("%s", m)
		default:
			return nil, fmt.Errorf("%v", resp)
		}
	}

	result := resp["data"].(map[string]interface{})

	bids, _ := result["BUY"].([]interface{})
	asks, _ := result["SELL"].([]interface{})

	dep := new(Depth)

	for _, v := range bids {
		r := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(r[0]), ToFloat64(r[1])})
	}

	for _, v := range asks {
		r := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(r[0]), ToFloat64(r[1])})
	}

	sort.Sort(sort.Reverse(dep.AskList))

	return dep, nil
}

func (ku *Kucoin) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, error) {
	panic("not implement")
}

//非个人，整个交易所的交易记录
func (ku *Kucoin) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implement")
}

func (ku *Kucoin) GetExchangeName() string {
	return "kucoin.com"
}

func (ku *Kucoin) getSign(endpoint string, params map[string]string) map[string]string {
	k := make(map[string]string)

	var keys []string

	for key, val := range params {
		keys = append(keys, key+`=`+val)
	}

	sort.Strings(keys)
	query := strings.Join(keys, `&`)

	k[`KC-API-NONCE`] = fmt.Sprintf("%d", time.Now().UTC().Unix()*1000)
	strForSign := endpoint + `/` + k[`KC-API-NONCE`] + `/` + query

	encoded := base64.URLEncoding.EncodeToString([]byte(strForSign))

	k[`KC-API-KEY`] = ku.apiKey
	k[`KC-API-SIGNATURE`], _ = GetParamHmacSHA256Sign(ku.secretKey, encoded)

	return k
}

func (ku *Kucoin) placeOrder(amount, price string, pair CurrencyPair, orderSide string) (*Order, error) {
	uri := fmt.Sprintf("%s/v1/order", ku.baseUrl)

	headers := ku.getSign(`/v1/order`, map[string]string{
		"symbol": pair.ToSymbol(`-`),
		"type":   orderSide,
		"price":  price,
		"amount": amount,
	})

	respBytes, err := HttpPostForm2(ku.client, uri, url.Values{
		"symbol": []string{pair.ToSymbol(`-`)},
		"type":   []string{orderSide},
		"price":  []string{price},
		"amount": []string{amount},
	}, headers)

	if err != nil {
		errCode := HTTP_ERR_CODE
		errCode.OriginErrMsg = err.Error()
		return nil, err
	}

	var resp map[string]interface{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	if val, ok := resp["success"]; ok && !val.(bool) {
		return nil, fmt.Errorf("%s", resp["msg"].(string))
	}

	res := resp["data"].(map[string]interface{})

	side := BUY
	if orderSide == "sell" {
		side = SELL
	}
	return &Order{
		Currency:   pair,
		OrderID2:   res[`orderOid`].(string),
		Price:      ToFloat64(price),
		Amount:     ToFloat64(amount),
		DealAmount: 0,
		AvgPrice:   0,
		Side:       TradeSide(side),
		Status:     ORDER_UNFINISH,
		OrderTime:  int(time.Now().Unix())}, nil
}
