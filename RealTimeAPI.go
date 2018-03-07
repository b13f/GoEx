package goex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/pkg/errors"
)

type RealTimeAPI interface {
	ListenDepth(pair CurrencyPair, depth chan *Depth) error
	ListenTrade(pair CurrencyPair, trade chan []Trade) error
}

const (
	DEPTH_CHANNEL = iota
	TRADE_CHANNEL
)

type ChannelFormatFunc func(pair CurrencyPair, channelType int) string
type SubChannelMessageFunc func(pair CurrencyPair, channelType int) interface{}

type RealTimeExchange struct {
	ChannelFormatFunc     ChannelFormatFunc
	SubChannelMessageFunc SubChannelMessageFunc

	c               *websocket.Conn
	depthChanMap    map[string]chan *Depth
	tradeChanMap    map[string]chan []Trade
	channelMap      map[string]int
	channelErrorMap map[string]chan error

	depthDataChanMap map[string]chan interface{}
	tradeDataChanMap map[string]chan interface{}

	writeMsgChan   chan interface{}
	writeErrorChan chan error
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewRealTimeExchange(channelFormatFunc ChannelFormatFunc, subChannelMessageFunc SubChannelMessageFunc) *RealTimeExchange {
	return &RealTimeExchange{
		ChannelFormatFunc:     channelFormatFunc,
		SubChannelMessageFunc: subChannelMessageFunc,
		tradeChanMap:          map[string]chan []Trade{},
		depthChanMap:          map[string]chan *Depth{},
		channelMap:            map[string]int{},
		writeMsgChan:          make(chan interface{}),
		writeErrorChan:        make(chan error),
		channelErrorMap:       map[string]chan error{},
		depthDataChanMap:      map[string]chan interface{}{},
		tradeDataChanMap:      map[string]chan interface{}{},
	}
}

func (realTimeExchange *RealTimeExchange) ListenDepth(pair CurrencyPair, depth chan *Depth) error {
	channel := realTimeExchange.ChannelFormatFunc(pair, DEPTH_CHANNEL)
	realTimeExchange.depthChanMap[channel] = depth
	realTimeExchange.channelMap[channel] = DEPTH_CHANNEL
	errorChan := make(chan error)
	realTimeExchange.channelErrorMap[channel] = errorChan
	depthDataChan := make(chan interface{})
	realTimeExchange.depthDataChanMap[channel] = depthDataChan

	msg := realTimeExchange.SubChannelMessageFunc(pair, DEPTH_CHANNEL)
	realTimeExchange.writeMsgChan <- msg

	if err := <-realTimeExchange.writeErrorChan; err != nil {
		return errors.Wrapf(err, "write depth msg %v", msg)
	}

	if err := <-realTimeExchange.channelErrorMap[channel]; err != nil {
		return errors.Wrapf(err, "sub %s channel failed", channel)
	}

	return nil
}

func (realTimeExchange *RealTimeExchange) ListenTrade(pair CurrencyPair, trade chan []Trade) error {
	channel := realTimeExchange.ChannelFormatFunc(pair, TRADE_CHANNEL)
	realTimeExchange.tradeChanMap[channel] = trade
	realTimeExchange.channelMap[channel] = TRADE_CHANNEL
	errorChan := make(chan error)
	realTimeExchange.channelErrorMap[channel] = errorChan
	tradeDataChan := make(chan interface{})
	realTimeExchange.tradeDataChanMap[channel] = tradeDataChan

	msg := realTimeExchange.SubChannelMessageFunc(pair, TRADE_CHANNEL)
	realTimeExchange.writeMsgChan <- msg

	if err := <-realTimeExchange.writeErrorChan; err != nil {
		return errors.Wrapf(err, "write trade msg %v", msg)
	}

	if err := <-realTimeExchange.channelErrorMap[channel]; err != nil {
		return errors.Wrapf(err, "sub %s channel failed", channel)
	}

	return nil
}

func (realTimeExchange *RealTimeExchange) connectWebsocket() error {
	url := "wss://real.okex.com:10440/websocket/okexapi"
	var err error
	realTimeExchange.c, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("websocket dial %s", url))
	}
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

func dataToTrades(data interface{}) ([]Trade, error) {
	var tradesdata []interface{}
	var trades []Trade
	var isok bool
	if tradesdata, isok = data.([]interface{}); !isok {
		return nil, errors.Errorf("data type is not []interface{} %v\n", tradesdata)
	}

	for _, d := range tradesdata {
		dd := d.([]interface{})
		t := Trade{}
		t.Tid = ToInt64(dd[0])
		t.Price = ToFloat64(dd[1])
		t.Amount = ToFloat64(dd[2])

		now := time.Now()
		timeStr := dd[3].(string)
		timeMeta := strings.Split(timeStr, ":")
		h, _ := strconv.Atoi(timeMeta[0])
		m, _ := strconv.Atoi(timeMeta[1])
		s, _ := strconv.Atoi(timeMeta[2])
		//临界点处理
		if now.Hour() == 0 {
			if h <= 23 && h >= 20 {
				pre := now.AddDate(0, 0, -1)
				t.Date = time.Date(pre.Year(), pre.Month(), pre.Day(), h, m, s, 0, time.Local).Unix() * 1000
			} else if h == 0 {
				t.Date = time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, time.Local).Unix() * 1000
			}
		} else {
			t.Date = time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, time.Local).Unix() * 1000
		}

		t.Type = dd[4].(string)
		trades = append(trades, t)
	}
	return trades, nil
}

func handleDepthData(ctx context.Context, depthDataChan chan interface{}, depthChan chan *Depth) {
	mergedDepth := &Depth{}
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-depthDataChan:
			d, err := dataToDepth(data)
			if err != nil {
				log.Printf("convert data to depth: %v\n", err)
				continue
			}
			mergedDepth = mergeDepth(mergedDepth, d)
			depthChan <- mergedDepth
		}
	}
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

func (realTimeExchange *RealTimeExchange) StopWebsocket() error {
	realTimeExchange.cancel()
	return nil
}

func (realTimeExchange *RealTimeExchange) RunWebsocket() error {
	ctx, cancel := context.WithCancel(context.Background())
	realTimeExchange.ctx = ctx
	realTimeExchange.cancel = cancel

	if err := realTimeExchange.connectWebsocket(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(time.Duration(5) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := realTimeExchange.c.WriteJSON(map[string]string{
					"event": "ping",
				})
				if err != nil {
					log.Printf("write ping: %v\n", err)
					if err := realTimeExchange.connectWebsocket(); err != nil {
						log.Printf("reconnect websocket failed: %v\n", err)
					}
				}
			case msg := <-realTimeExchange.writeMsgChan:
				realTimeExchange.writeErrorChan <- realTimeExchange.c.WriteJSON(msg)
			case <-ctx.Done():
				close(realTimeExchange.writeMsgChan)
				return
			}
		}
	}()

	msgChan := make(chan []byte)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msg, err := realTimeExchange.c.ReadMessage()
				if err != nil {
					log.Printf("read message: %v\n", err)
				}
				if string(msg) == "{\"event\":\"pong\"}" {
					continue
				}
				msgChan <- msg
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(msgChan)
				return
			case msg := <-msgChan:
				var msgmaplist []map[string]interface{}

				if err := json.Unmarshal(msg, &msgmaplist); err != nil {
					log.Printf("unmarshal: %v msg: %s\n", string(msg), err)
					continue
				}

				if len(msgmaplist) == 0 {
					log.Printf("length of msg is zero.")
					continue
				}

				for _, msgmap := range msgmaplist {
					var isok bool
					var channel string
					var channelType int

					if channel, isok = msgmap["channel"].(string); !isok {
						log.Printf("miss channel, msg map: %v\n", msgmap)
						continue
					}

					if channel == "addChannel" {
						data := msgmap["data"].(map[string]interface{})
						channelName := data["channel"].(string)
						errorChan := realTimeExchange.channelErrorMap[channelName]
						if result := data["result"].(bool); !result {
							errorChan <- errors.Errorf("add channel failed error code(%v)", data["errorcode"])
						} else {
							if channelType, isok = realTimeExchange.channelMap[channelName]; !isok {
								errorChan <- errors.Errorf("receive unknown channel(%s) message(%v)\n", channel, msgmap)
							} else {
								if channelType == DEPTH_CHANNEL {
									go handleDepthData(ctx, realTimeExchange.depthDataChanMap[channelName], realTimeExchange.depthChanMap[channelName])
								} else if channelType == TRADE_CHANNEL {
									go handleTradeData(ctx, realTimeExchange.tradeDataChanMap[channelName], realTimeExchange.tradeChanMap[channelName])
								}
								errorChan <- nil
							}
						}
						continue
					}

					if channelType, isok = realTimeExchange.channelMap[channel]; !isok {
						log.Printf("receive unknown channel(%s) message(%v)\n", channel, msgmap)
						continue
					}

					if channelType == DEPTH_CHANNEL {
						realTimeExchange.depthDataChanMap[channel] <- msgmap["data"]
					} else if channelType == TRADE_CHANNEL {
						realTimeExchange.tradeDataChanMap[channel] <- msgmap["data"]
					}
				}
			}
		}
	}()

	return nil
}

func mergeDepth(depth, d *Depth) *Depth {
	merge := func(list, l DepthRecords) DepthRecords {
		for _, a := range l {
			processed := false
			for i, ask := range list {
				if a.Price < ask.Price {
					continue
				} else if a.Price == ask.Price {
					if a.Amount == 0 {
						list = append(list[:i], list[i+1:]...)
					} else {
						list[i].Amount = a.Amount
					}
					processed = true
					break
				} else {
					list = append(list, DepthRecord{})
					copy(list[i+1:], list[i:])
					list[i] = a
					processed = true
					break
				}
			}
			if processed == false {
				list = append(list, a)
			}
		}
		return list
	}
	depth.AskList = merge(depth.AskList, d.AskList)
	depth.BidList = merge(depth.BidList, d.BidList)
	return depth
}
