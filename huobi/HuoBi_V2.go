package huobi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	. "github.com/nntaoli-project/GoEx"
	"net/http"
	"net/url"
	"strings"
	"time"
	"log"

	"github.com/pkg/errors"
)

type HuoBi_V2 struct {
	*RealTimeExchange
	httpClient *http.Client
	accountId,
	baseUrl,
	accessKey,
	secretKey string
}

type response struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Errmsg  string          `json:"err-msg"`
	Errcode string          `json:"err-code"`
}

func NewV2(httpClient *http.Client, accessKey, secretKey, clientId string) *HuoBi_V2 {
	hb := &HuoBi_V2{
		httpClient: httpClient,
		accountId:  clientId,
		baseUrl:    "https://be.huobi.com",
		accessKey:  accessKey,
		secretKey:  secretKey,
	}
	hb.RealTimeExchange = NewRealTimeExchange(hb)
	return hb
}

func (hbV2 *HuoBi_V2) GetAccountId() (string, error) {
	path := "/v1/account/accounts"
	params := &url.Values{}
	hbV2.buildPostForm("GET", path, params)

	//log.Println(hbV2.baseUrl + path + "?" + params.Encode())

	respmap, err := HttpGet(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return "", err
	}
	//log.Println(respmap)
	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	data := respmap["data"].([]interface{})
	accountIdMap := data[0].(map[string]interface{})
	hbV2.accountId = fmt.Sprintf("%.f", accountIdMap["id"].(float64))

	//log.Println(respmap)
	return hbV2.accountId, nil
}

func (hbV2 *HuoBi_V2) GetAccount() (*Account, error) {
	path := fmt.Sprintf("/v1/account/accounts/%s/balance", hbV2.accountId)
	params := &url.Values{}
	params.Set("accountId-id", hbV2.accountId)
	hbV2.buildPostForm("GET", path, params)

	urlStr := hbV2.baseUrl + path + "?" + params.Encode()
	//println(urlStr)
	respmap, err := HttpGet(hbV2.httpClient, urlStr)

	if err != nil {
		return nil, err
	}

	//log.Println(respmap)

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	if datamap["state"].(string) != "working" {
		return nil, errors.New(datamap["state"].(string))
	}

	list := datamap["list"].([]interface{})
	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount, 6)
	acc.Exchange = hbV2.GetExchangeName()

	subAccMap := make(map[Currency]*SubAccount)

	for _, v := range list {
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["currency"].(string)
		currency := NewCurrency(currencySymbol)
		typeStr := balancemap["type"].(string)
		balance := ToFloat64(balancemap["balance"])
		if subAccMap[currency] == nil {
			subAccMap[currency] = new(SubAccount)
		}
		subAccMap[currency].Currency = currency
		switch typeStr {
		case "trade":
			subAccMap[currency].Amount = balance
		case "frozen":
			subAccMap[currency].ForzenAmount = balance
		}
	}

	for k, v := range subAccMap {
		acc.SubAccounts[k] = *v
	}

	return acc, nil
}

func (hbV2 *HuoBi_V2) placeOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	path := "/v1/order/orders/place"
	params := url.Values{}
	params.Set("account-id", hbV2.accountId)
	params.Set("amount", amount)
	params.Set("symbol", strings.ToLower(pair.ToSymbol("")))
	params.Set("type", orderType)

	switch orderType {
	case "buy-limit", "sell-limit":
		params.Set("price", price)
	}

	hbV2.buildPostForm("POST", path, &params)

	resp, err := HttpPostForm3(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode(), hbV2.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return "", err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return "", err
	}

	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	return respmap["data"].(string), nil
}

func (hbV2 *HuoBi_V2) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "buy-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Pair: currency,
		Id:  orderId,
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY,
		OrderTime: time.Now()}, nil
}

func (hbV2 *HuoBi_V2) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "sell-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Pair: currency,
		Id:  orderId,
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL,
		OrderTime: time.Now()}, nil
}

func (hbV2 *HuoBi_V2) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "buy-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Pair: currency,
		Id:  orderId,
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY_MARKET,
		OrderTime: time.Now()}, nil
}

func (hbV2 *HuoBi_V2) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "sell-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Pair: currency,
		Id:  orderId,
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL_MARKET,
		OrderTime: time.Now()}, nil
}

func (hbV2 *HuoBi_V2) parseOrder(ordmap map[string]interface{}) Order {
	ord := Order{
		Id:    fmt.Sprintf("%.0f",ordmap["id"].(float64)),
		Amount:     ToFloat64(ordmap["amount"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["field-amount"]),
		Fee:        ToFloat64(ordmap["field-fees"]),
		OrderTime:  time.Unix(int64(ordmap["created-at"].(float64)/1000),((int64(ordmap["created-at"].(float64)))%1000)*1000*1000),
	}

	state := ordmap["state"].(string)
	switch state {
	case "submitted":
		ord.Status = ORDER_UNFINISH
	case "filled":
		ord.Status = ORDER_FINISH
	case "partial-filled":
		ord.Status = ORDER_PART_FINISH
	case "canceled", "partial-canceled":
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	if ord.DealAmount > 0.0 {
		ord.AvgPrice = ToFloat64(ordmap["field-cash-amount"]) / ord.DealAmount
	}

	typeS := ordmap["type"].(string)
	switch typeS {
	case "buy-limit":
		ord.Side = BUY
	case "buy-market":
		ord.Side = BUY_MARKET
	case "sell-limit":
		ord.Side = SELL
	case "sell-market":
		ord.Side = SELL_MARKET
	}
	return ord
}

func (hbV2 *HuoBi_V2) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	path := "/v1/order/orders/" + orderId
	params := url.Values{}
	hbV2.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	order := hbV2.parseOrder(datamap)
	order.Pair = currency
	//log.Println(respmap)
	return &order, nil
}

func (hbV2 *HuoBi_V2) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	return hbV2.getOrders(queryOrdersParams{
		pair:   currency,
		states: "pre-submitted,submitted,partial-filled",
		size:   100,
		//direct:""
	})
}

func (hbV2 *HuoBi_V2) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	path := fmt.Sprintf("/v1/order/orders/%s/submitcancel", orderId)
	params := url.Values{}
	hbV2.buildPostForm("POST", path, &params)
	resp, err := HttpPostForm3(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode(), hbV2.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return false, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	if respmap["status"].(string) != "ok" {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (hbV2 *HuoBi_V2) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	return hbV2.getOrders(queryOrdersParams{
		pair:   currency,
		size:   pageSize,
		states: "partial-canceled,filled",
		direct: "next",
	})
}

type queryOrdersParams struct {
	types,
	startDate,
	endDate,
	states,
	from,
	direct string
	size int
	pair CurrencyPair
}

func (hbV2 *HuoBi_V2) getOrders(queryparams queryOrdersParams) ([]Order, error) {
	path := "/v1/order/orders"
	params := url.Values{}
	params.Set("symbol", strings.ToLower(queryparams.pair.ToSymbol("")))
	params.Set("states", queryparams.states)

	if queryparams.direct != "" {
		params.Set("direct", queryparams.direct)
	}

	if queryparams.size > 0 {
		params.Set("size", fmt.Sprint(queryparams.size))
	}

	hbV2.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf("%s%s?%s", hbV2.baseUrl, path, params.Encode()))
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].([]interface{})
	var orders []Order
	for _, v := range datamap {
		ordmap := v.(map[string]interface{})
		ord := hbV2.parseOrder(ordmap)
		ord.Pair = queryparams.pair
		orders = append(orders, ord)
	}

	return orders, nil
}

func (hbV2 *HuoBi_V2) GetExchangeName() string {
	return "huobi.com"
}

func (hbV2 *HuoBi_V2) GetTicker(currencyPair CurrencyPair) (*Ticker, error) {
	url := hbV2.baseUrl + "/market/detail/merged?symbol=" + strings.ToLower(currencyPair.ToSymbol(""))
	respmap, err := HttpGet(hbV2.httpClient, url)
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) == "error" {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	tickmap, ok := respmap["tick"].(map[string]interface{})
	if !ok {
		return nil, errors.New("tick assert error")
	}

	ticker := new(Ticker)
	ticker.Vol = ToFloat64(tickmap["amount"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.High = ToFloat64(tickmap["high"])
	bid, isOk := tickmap["bid"].([]interface{})
	if isOk != true {
		return nil, errors.New("no bid")
	}
	ask, isOk := tickmap["ask"].([]interface{})
	if isOk != true {
		return nil, errors.New("no ask")
	}
	ticker.Buy = ToFloat64(bid[0])
	ticker.Sell = ToFloat64(ask[0])
	ticker.Last = ToFloat64(tickmap["close"])
	ticker.Date = ToUint64(respmap["ts"])

	return ticker, nil
}

func (hbV2 *HuoBi_V2) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	url := hbV2.baseUrl + "/market/depth?symbol=%s&type=step0"
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf(url, strings.ToLower(currency.ToSymbol(""))))
	if err != nil {
		return nil, err
	}

	if "ok" != respmap["status"].(string) {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	tick, _ := respmap["tick"].(map[string]interface{})
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
	_size := size
	for _, r := range asks {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.AskList = append(depth.AskList, dr)

		_size--
		if _size == 0 {
			break
		}
	}

	_size = size
	for _, r := range bids {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.BidList = append(depth.BidList, dr)

		_size--
		if _size == 0 {
			break
		}
	}

	return depth, nil
}

func (hbV2 *HuoBi_V2) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, error) {
	panic("not implement")
}

//非个人，整个交易所的交易记录
func (hbV2 *HuoBi_V2) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implement")
}

func (hbV2 *HuoBi_V2) buildPostForm(reqMethod, path string, postForm *url.Values) error {
	postForm.Set("AccessKeyId", hbV2.accessKey)
	postForm.Set("SignatureMethod", "HmacSHA256")
	postForm.Set("SignatureVersion", "2")
	postForm.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(hbV2.baseUrl, "https://", "", len(hbV2.baseUrl))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", reqMethod, domain, path, postForm.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(hbV2.secretKey, payload)
	postForm.Set("Signature", sign)
	return nil
}

func (hbV2 *HuoBi_V2) toJson(params url.Values) string {
	parammap := make(map[string]string)
	for k, v := range params {
		parammap[k] = v[0]
	}
	jsonData, _ := json.Marshal(parammap)
	return string(jsonData)
}

func (hb *HuoBi_V2) GenChannel(pair CurrencyPair, channelType int) string {
	var suffix string
	switch channelType {
	case DEPTH_CHANNEL:
		suffix = "depth.step0"
	case TRADE_CHANNEL:
		suffix = "trade.detail"
	}
	return fmt.Sprintf("market.%s.%s", strings.ToLower(pair.ToSymbol("")), suffix)
}

func (hb *HuoBi_V2) GenSubMessage(channel string) interface{} {
	return map[string]interface{}{
		"sub": channel,
		"id":  channel,
	}
}

func (hb *HuoBi_V2) GetWebsocketURL() string {
	return "wss://api.huobi.pro/ws"
}

func (hb *HuoBi_V2) GetKeepAliveHandler() KeepAliveHandler {
	return nil
}

func dataToDepth(data interface{}) (*Depth, error) {
	var isok bool
	var bids, asks []interface{}
	var depthdata map[string]interface{}
	if depthdata, isok = data.(map[string]interface{}); !isok {
		return nil, errors.Errorf("data type is not map[string]interface{}: %v\n", data)
	}
	bids, _ = depthdata["bids"].([]interface{})
	asks, _ = depthdata["asks"].([]interface{})

	d := new(Depth)
	for _, r := range asks {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		d.AskList = append(d.AskList, dr)
	}

	for _, r := range bids {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		d.BidList = append(d.BidList, dr)
	}
	return d, nil
}

func handleDepthData(ctx context.Context, depthDataChan chan interface{}, depthChan chan *Depth) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-depthDataChan:
			d, err := dataToDepth(data)
			// 火币拿到的ask列表是从小到大排列的，需要反转过来
			//for i, j := 0, len(d.AskList)-1; i < j; i, j = i+1, j-1 {
			//	d.AskList[i], d.AskList[j] = d.AskList[j], d.AskList[i]
			//}
			if err != nil {
				log.Printf("convert data to depth: %v\n", err)
				continue
			}
			depthChan <- d
		}
	}
}

func dataToTrades(data interface{}) ([]Trade, error) {
	tradeData, isok := data.([]interface{})
	if !isok {
		return nil, errors.Errorf("data type is not []interface{}: %v\n", data)
	}
	var trades []Trade
	for _, t := range tradeData {
		tr := t.(map[string]interface{})
		trade := Trade{}
		trade.Amount = tr["amount"].(float64)
		trade.Price = tr["price"].(float64)
		if tr["direction"].(string) == "buy" {
			trade.Type = "bid"
		} else {
			trade.Type = "ask"
		}
		//trade.Tid = int64(tr["id"].(float64))
		trade.Date = int64(tr["ts"].(float64))
		trades = append(trades, trade)
	}
	return trades, nil
}

func handleTradeData(ctx context.Context, tradeDataChan chan interface{}, tradeChan chan []Trade) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-tradeDataChan:
			d, err := dataToTrades(data)
			if err != nil {
				log.Printf("convert data to depth: %v\n", err)
				continue
			}
			tradeChan <- d
		}
	}
}

func (hb *HuoBi_V2) GetMessageHandler() MessageHandler {
	return func(ctx context.Context, realTimeExchange *RealTimeExchange, msgChan chan []byte) {
		depthDataChanMap := map[string]chan interface{}{}
		tradeDataChanMap := map[string]chan interface{}{}
		for {
			select {
			case <-ctx.Done():
				close(msgChan)
				return
			case msg := <-msgChan:
				reader := bytes.NewReader(msg)
				gzipReader, err := gzip.NewReader(reader)
				if err != nil {
					log.Printf("new gzip reader: %v\n", err)
					continue
				}
				jsonReader := json.NewDecoder(gzipReader)

				var msgmap map[string]interface{}
				if err := jsonReader.Decode(&msgmap); err != nil {
					log.Printf("decode json: %v\n", err)
					continue
				}

				if ping, isok := msgmap["ping"]; isok {
					err := realTimeExchange.WriteJSONMessage(map[string]interface{}{"pong": ping})
					if err != nil {
						log.Printf("write pong %v\n", err)
					}
					continue
				}

				if channelName, isok := msgmap["id"].(string); isok {
					if errorChan, err := realTimeExchange.GetSubChannelErrorChan(channelName); err != nil {
						log.Printf("unknown channel %s\n", channelName)
					} else {
						if result := msgmap["status"].(string); result != "ok" {
							errorChan <- errors.Errorf("add channel failed error code(%v)", msgmap["err-msg"])
						} else {
							if channelType, err := realTimeExchange.GetChannelType(channelName); err != nil {
								errorChan <- errors.Wrap(err, "get channel type")
							} else {
								if channelType == DEPTH_CHANNEL {
									if depthChan, err := realTimeExchange.GetDepthChan(channelName); err != nil {
										errorChan <- errors.Wrap(err, "get depth chan")
									} else {
										depthDataChanMap[channelName] = make(chan interface{})
										go handleDepthData(ctx, depthDataChanMap[channelName], depthChan)
									}
								} else if channelType == TRADE_CHANNEL {
									if tradeChan, err := realTimeExchange.GetTradeChan(channelName); err != nil {
										errorChan <- errors.Wrap(err, "get trade chan")
									} else {
										tradeDataChanMap[channelName] = make(chan interface{})
										go handleTradeData(ctx, tradeDataChanMap[channelName], tradeChan)
									}
								}
								errorChan <- nil
							}
						}
					}
					continue
				}

				if channelName, isok := msgmap["ch"].(string); isok {
					if channelType, err := realTimeExchange.GetChannelType(channelName); err != nil {
						log.Printf("receive unknown channel(%s) message(%v)\n", channelName, msgmap)
					} else {
						if channelType == DEPTH_CHANNEL {
							if data, isok := msgmap["tick"]; isok {
								depthDataChanMap[channelName] <- data
							} else {
								log.Printf("miss tick in msg %v\n", msgmap)
							}
						} else if channelType == TRADE_CHANNEL {
							if tick, isok := msgmap["tick"]; isok {
								if data, isok := tick.(map[string]interface{})["data"]; isok {
									tradeDataChanMap[channelName] <- data
								} else {
									log.Printf("miss data in tick %v\n", msgmap)
								}
							} else {
								log.Printf("miss tick in msg %v\n", msgmap)
							}
						}
					}
					continue
				}

				log.Println("unknown msgmap:", msgmap)
			}
		}
	}
}

func (hb *HuoBi_V2) GetCurrencies() ([]Currency, error) {
	currencyUrl := hb.baseUrl + "/v1/common/currencys"
	body, err := HttpGet(hb.httpClient, currencyUrl)
	if err != nil {
		return nil, err
	}

	var currencies []Currency
	data := body["data"].([]interface{})
	for _, d := range data {
		c := Currency{Symbol: strings.ToUpper(d.(string))}
		currencies = append(currencies, c)
	}

	return currencies, nil
}

func (hb *HuoBi_V2) DepthSubscribe(pair CurrencyPair) (chan *Depth, error) {
	dCh := make(chan *Depth)

	err := hb.RealTimeExchange.RunWebsocket()
	if err != nil {
		return nil, err
	}

	err = hb.RealTimeExchange.ListenDepth(pair,dCh)

	return dCh, err
}

func (hb *HuoBi_V2) GetOrdersChan() (chan *Order, error) {
	return nil,fmt.Errorf("not implemented")
}
