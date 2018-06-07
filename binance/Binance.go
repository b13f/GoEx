package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	. "github.com/nntaoli-project/GoEx"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	EXCHANGE_NAME = "binance.com"

	API_BASE_URL = "https://api.binance.com/"
	API_V1       = API_BASE_URL + "api/v1/"
	API_V3       = API_BASE_URL + "api/v3/"

	TICKER_URI             = "ticker/24hr?symbol=%s"
	TICKERS_URI            = "ticker/allBookTickers"
	DEPTH_URI              = "depth?symbol=%s&limit=%d"
	ACCOUNT_URI            = "account?"
	ORDER_URI              = "order?"
	UNFINISHED_ORDERS_INFO = "openOrders?"
)

type Binance struct {
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

func (bn *Binance) buildParamsSigned(postForm *url.Values) error {
	postForm.Set("recvWindow", "6000000")
	tonce := strconv.FormatInt(time.Now().UnixNano(), 10)[0:13]
	postForm.Set("timestamp", tonce)
	payload := postForm.Encode()
	sign, _ := GetParamHmacSHA256Sign(bn.secretKey, payload)
	postForm.Set("signature", sign)
	return nil
}

func New(client *http.Client, api_key, secret_key string) *Binance {
	return &Binance{api_key, secret_key, client}
}

func (bn *Binance) GetExchangeName() string {
	return EXCHANGE_NAME
}

func (bn *Binance) GetTicker(currency CurrencyPair) (*Ticker, error) {
	tickerUri := API_V1 + fmt.Sprintf(TICKER_URI, currency.ToSymbol(""))
	tickerMap, err := HttpGet(bn.httpClient, tickerUri)

	if err != nil {
		log.Println("GetTicker error:", err)
		return nil, err
	}

	var ticker Ticker

	t, _ := tickerMap["closeTime"].(float64)
	ticker.Date = uint64(t / 1000)
	ticker.Last = ToFloat64(tickerMap["lastPrice"])
	ticker.Buy = ToFloat64(tickerMap["bidPrice"])
	ticker.Sell = ToFloat64(tickerMap["askPrice"])
	ticker.Low = ToFloat64(tickerMap["lowPrice"])
	ticker.High = ToFloat64(tickerMap["highPrice"])
	ticker.Vol = ToFloat64(tickerMap["volume"])
	return &ticker, nil
}

func (bn *Binance) GetDepth(size int, currencyPair CurrencyPair) (*Depth, error) {
	if size > 100 {
		size = 100
	} else if size < 5 {
		size = 5
	}

	apiUrl := fmt.Sprintf(API_V1+DEPTH_URI, currencyPair.ToSymbol(""), size)
	resp, err := HttpGet(bn.httpClient, apiUrl)
	if err != nil {
		log.Println("GetDepth error:", err)
		return nil, err
	}

	if _, isok := resp["code"]; isok {
		return nil, errors.New(resp["msg"].(string))
	}

	bids := resp["bids"].([]interface{})
	asks := resp["asks"].([]interface{})

	//log.Println(bids)
	//log.Println(asks)

	depth := new(Depth)

	for _, bid := range bids {
		_bid := bid.([]interface{})
		amount := ToFloat64(_bid[1])
		price := ToFloat64(_bid[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.BidList = append(depth.BidList, dr)
	}

	for _, ask := range asks {
		_ask := ask.([]interface{})
		amount := ToFloat64(_ask[1])
		price := ToFloat64(_ask[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.AskList = append(depth.AskList, dr)
	}

	return depth, nil
}

func (bn *Binance) placeOrder(amount, price string, pair CurrencyPair, orderType, orderSide string) (*Order, error) {
	path := API_V3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", pair.ToSymbol(""))
	params.Set("side", orderSide)
	params.Set("type", orderType)

	params.Set("quantity", amount)
	params.Set("type", "LIMIT")
	params.Set("timeInForce", "GTC")

	switch orderType {
	case "LIMIT":
		params.Set("price", price)
	}

	bn.buildParamsSigned(&params)

	resp, err := HttpPostForm2(bn.httpClient, path, params,
		map[string]string{"X-MBX-APIKEY": bn.accessKey})
	//log.Println("resp:", string(resp), "err:", err)
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		log.Println(string(resp))
		return nil, err
	}

	orderId := ToInt(respmap["orderId"])
	if orderId <= 0 {
		return nil, errors.New(string(resp))
	}

	side := BUY
	if orderSide == "SELL" {
		side = SELL
	}

	return &Order{
		Pair:       pair,
		Id:         fmt.Sprintf("%d", orderId),
		Price:      ToFloat64(price),
		Amount:     ToFloat64(amount),
		DealAmount: 0,
		AvgPrice:   0,
		Side:       TradeSide(side),
		Status:     ORDER_UNFINISH,
		OrderTime:  time.Now()}, nil
}

func (bn *Binance) GetAccount() (*Account, error) {
	params := url.Values{}
	bn.buildParamsSigned(&params)
	path := API_V3 + ACCOUNT_URI + params.Encode()
	respmap, err := HttpGet2(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//log.Println("respmap:", respmap)
	if _, isok := respmap["code"]; isok == true {
		return nil, errors.New(respmap["msg"].(string))
	}
	acc := Account{}
	acc.Exchange = bn.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		//log.Println(v)
		vv := v.(map[string]interface{})

		if ToFloat64(vv["free"]) == 0 && ToFloat64(vv["locked"]) == 0 {
			continue
		}

		currency := NewCurrency(vv["asset"].(string))
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["free"]),
			ForzenAmount: ToFloat64(vv["locked"]),
		}
	}

	return &acc, nil
}

func (bn *Binance) LimitBuy(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return bn.placeOrder(amount, price, currencyPair, "LIMIT", "BUY")
}

func (bn *Binance) LimitSell(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return bn.placeOrder(amount, price, currencyPair, "LIMIT", "SELL")
}

func (bn *Binance) MarketBuy(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return bn.placeOrder(amount, price, currencyPair, "MARKET", "BUY")
}

func (bn *Binance) MarketSell(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return bn.placeOrder(amount, price, currencyPair, "MARKET", "SELL")
}

func (bn *Binance) CancelOrder(orderId string, currencyPair CurrencyPair) (bool, error) {
	path := API_V3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))
	params.Set("orderId", orderId)

	bn.buildParamsSigned(&params)

	resp, err := HttpDeleteForm(bn.httpClient, path, params, map[string]string{"X-MBX-APIKEY": bn.accessKey})

	//log.Println("resp:", string(resp), "err:", err)
	if err != nil {
		return false, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		log.Println(string(resp))
		return false, err
	}

	orderIdCanceled := ToInt(respmap["orderId"])
	if orderIdCanceled <= 0 {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (bn *Binance) GetOneOrder(orderId string, currencyPair CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))
	if orderId != "" {
		params.Set("orderId", orderId)
	}
	params.Set("orderId", orderId)

	bn.buildParamsSigned(&params)
	path := API_V3 + ORDER_URI + params.Encode()

	respmap, err := HttpGet2(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	//log.Println(respmap)
	if err != nil {
		return nil, err
	}
	status := respmap["status"].(string)
	side := respmap["side"].(string)

	ord := Order{}
	ord.Pair = currencyPair
	ord.Id = orderId

	if side == "SELL" {
		ord.Side = SELL
	} else {
		ord.Side = BUY
	}

	switch status {
	case "FILLED":
		ord.Status = ORDER_FINISH
	case "PARTIALLY_FILLED":
		ord.Status = ORDER_PART_FINISH
	case "CANCELED":
		ord.Status = ORDER_CANCEL
	case "PENDING_CANCEL":
		ord.Status = ORDER_CANCEL_ING
	case "REJECTED":
		ord.Status = ORDER_REJECT
	}

	ord.Amount = ToFloat64(respmap["origQty"].(string))
	ord.Price = ToFloat64(respmap["price"].(string))
	ord.DealAmount = ToFloat64(respmap["executedQty"])
	ord.AvgPrice = ord.Price // response no avg price ， fill price
	ord.OrderTime = time.Unix(int64(respmap["time"].(float64)/1000), ((int64(respmap["time"].(float64)))%1000)*1000*1000)

	return &ord, nil
}

func (bn *Binance) GetUnfinishOrders(currencyPair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))

	bn.buildParamsSigned(&params)
	path := API_V3 + UNFINISHED_ORDERS_INFO + params.Encode()

	respmap, err := HttpGet3(bn.httpClient, path, map[string]string{"X-MBX-APIKEY": bn.accessKey})
	//log.Println("respmap", respmap, "err", err)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range respmap {
		ord := v.(map[string]interface{})
		side := ord["side"].(string)
		orderSide := SELL
		if side == "BUY" {
			orderSide = BUY
		}

		orders = append(orders, Order{
			Id:        fmt.Sprintf("%f",ord["orderId"].(float64)),
			Pair:      currencyPair,
			Price:     ToFloat64(ord["price"]),
			Amount:    ToFloat64(ord["origQty"]),
			Side:      TradeSide(orderSide),
			Status:    ORDER_UNFINISH,
			OrderTime: time.Unix(int64(ord["time"].(float64)/1000), ((int64(ord["time"].(float64)))%1000)*1000*1000)})
	}
	return orders, nil
}

func (bn *Binance) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, error) {
	panic("not implements")
}

//非个人，整个交易所的交易记录
func (bn *Binance) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implements")
}

func (bn *Binance) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	panic("not implements")
}

func (bn *Binance) DepthSubscribe(pair CurrencyPair) (chan *Depth, error) {
	url := fmt.Sprintf(`wss://stream.binance.com:9443/ws/%s@depth5`, strings.ToLower(pair.ToSymbol(``)))
	c, err := bn.wsConnect(url)
	if err != nil {
		return nil, fmt.Errorf("websocket dial error %s", err)
	}

	msgChan := make(chan *Depth)

	go func() {
		for {
			select {
			default:
				_, msg, err := c.ReadMessage()
				if err != nil {
					log.Printf("read message: %v\n", err)
					if _, isClose := err.(*websocket.CloseError); isClose == true {
						//reconnect
						c, err = bn.wsConnect(url)
					}
					continue
				}

				msgChan <- bn.wsDepthDecode(msg)
			}
		}
	}()

	return msgChan, nil
}

func (bn *Binance) wsConnect(url string) (*websocket.Conn, error) {
	d := &websocket.Dialer{}
	c, _, err := d.Dial(url, nil)
	if err != nil {
		//return nil,fmt.Errorf("websocket dial error %s", err)
	}
	return c, err
}

func (bn *Binance) wsDepthDecode(m []byte) *Depth {
	respmap := struct {
		LastUpdateId int
		Bids         [][]interface{}
		Asks         [][]interface{}
	}{}

	err := json.Unmarshal(m, &respmap)
	if err != nil {
		//
	}

	d := Depth{AskList: DepthRecords{}, BidList: DepthRecords{}}

	for _, t := range respmap.Asks {
		p, _ := strconv.ParseFloat(t[0].(string), 64)
		a, _ := strconv.ParseFloat(t[1].(string), 64)
		d.AskList = append(d.AskList, DepthRecord{
			Price:  p,
			Amount: a,
		})
	}
	for _, t := range respmap.Bids {
		p, _ := strconv.ParseFloat(t[0].(string), 64)
		a, _ := strconv.ParseFloat(t[1].(string), 64)
		d.BidList = append(d.BidList, DepthRecord{
			Price:  p,
			Amount: a,
		})
	}

	return &d
}

func (bn *Binance) getStreamKey() (streamKey string, err error) {
	resp, err := HttpPostForm2(bn.httpClient, API_V1 +`userDataStream`,
		nil,map[string]string{"X-MBX-APIKEY": bn.accessKey})
	//log.Println("resp:", string(resp), "err:", err)
	if err != nil {
		return
	}

	t := struct {
		ListenKey string
	}{}

	err = json.Unmarshal(resp,&t)
	if err != nil {
		return
	}

	return t.ListenKey,nil
}

func (bn *Binance) GetOrdersChan() (chan *Order, error) {
	msgT := struct {
		E	string `json:"e"`
		Eu	int64  `json:"E"`
	}{}

	//msgB := struct {
	//	M  int   `json:"m"`
	//	T  int   `json:"t"`
	//	B  int   `json:"b"`
	//	S  int   `json:"s"`
	//	Tu bool  `json:"T"`
	//	Wu bool  `json:"W"`
	//	Du bool  `json:"D"`
	//	U  int64 `json:"u"`
	//	Bu []struct {
	//		A string `json:"a"`
	//		F string `json:"f"`
	//		L string `json:"l"`
	//	} `json:"B"`
	//}{}
	msgO := struct {
		Symbol		string `json:"s"`
		ClientID	string `json:"c"`
		Side		string `json:"S"`
		OrderType	string `json:"o"`
		TimeForce	string `json:"f"`
		Amount		string `json:"q"`
		Price		string `json:"p"`
		StopPrice	string `json:"P"`
		IcebergAmount	string `json:"F"`
		ClientOriginalID	string `json:"C"`
		ExecType	string `json:"x"`
		Status		string `json:"X"`
		RejectReason	string `json:"r"`
		OrderID		int `json:"i"`
		LastAmount	string `json:"l"`
		CumAmount	string `json:"z"`
		LastPrice	string `json:"L"`
		FeeAmount	string `json:"n"`
		FeeAsset	string `json:"N"`
		Time		int64 `json:"T"`
		TradeID		int `json:"t"`
		IsWorking	bool `json:"w"`
		IsMaker		bool `json:"m"`
		Ignore		int `json:"I"`
	}{}

	key, err := bn.getStreamKey()
	if err != nil {
		return nil,err
	}

	url := fmt.Sprintf(`wss://stream.binance.com:9443/ws/%s`, key)
	c, err := bn.wsConnect(url)
	if err != nil {
		return nil,err
	}

	msgChan := make(chan *Order)

	go func() {
		for {
			select {
			default:
				_, msg, err := c.ReadMessage()
				if err != nil {
					log.Printf("read message: %v\n", err)
					if _, isClose := err.(*websocket.CloseError); isClose == true {
						//reconnect
						key, _ := bn.getStreamKey()
						url := fmt.Sprintf(`wss://stream.binance.com:9443/ws/%s`, key)
						c, err = bn.wsConnect(url)
					}
					continue
				}

				json.Unmarshal(msg,&msgT)

				if msgT.E == "executionReport" {
					json.Unmarshal(msg,&msgO)

				}

				if msgO.ExecType != "TRADE" {
					continue
				}

				//fmt.Printf("STATUS OF GETTING ORDER IS %s and %s\n",msgO.Status,msgO.ExecType)
				o := &Order{Id:fmt.Sprintf("%d",msgO.OrderID)}

				if msgO.Side == "SELL" {
					o.Side = SELL
				} else {
					o.Side = BUY
				}

				o.Amount = ToFloat64(msgO.Amount)
				o.Price = ToFloat64(msgO.Price)

				msgChan <- o
				//select {
				//case msgChan <- o:
				//default:
				//}

			}
		}
	}()

	return msgChan,nil
}