package goex

import (
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"github.com/pkg/errors"
)

type KeepAliveHandler func(context.Context, *RealTimeExchange)
type MessageHandler func(context.Context, *RealTimeExchange, chan []byte)

type RealTimeProtocol interface {
	GenChannel(pair CurrencyPair, channelType int) string
	GenSubMessage(channel string) interface{}
	GetWebsocketURL() string
	GetKeepAliveHandler() KeepAliveHandler
	GetMessageHandler() MessageHandler
}

const (
	UNKNOWN_CHANNEL = -1
	DEPTH_CHANNEL   = iota
	TRADE_CHANNEL
)

type RealTimeExchange struct {
	exchange RealTimeProtocol

	c                  *websocket.Conn
	depthChanMap       map[string]chan *Depth
	tradeChanMap       map[string]chan []Trade
	subChannelErrorMap map[string]chan error
	channelTypeMap     map[string]int

	writeMsgChan   chan interface{}
	writeErrorChan chan error
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewRealTimeExchange(exchange RealTimeProtocol) *RealTimeExchange {
	return &RealTimeExchange{
		exchange:           exchange,
		tradeChanMap:       map[string]chan []Trade{},
		depthChanMap:       map[string]chan *Depth{},
		subChannelErrorMap: map[string]chan error{},
		channelTypeMap:     map[string]int{},

		writeMsgChan:   make(chan interface{}),
		writeErrorChan: make(chan error),
	}
}

func (realTimeExchange *RealTimeExchange) GetChannelType(channel string) (int, error) {
	if channelType, isok := realTimeExchange.channelTypeMap[channel]; isok {
		return channelType, nil
	} else {
		return UNKNOWN_CHANNEL, errors.Errorf("unknown channel %s", channel)
	}
}

func (realTimeExchange *RealTimeExchange) GetSubChannelErrorChan(channel string) (chan error, error) {
	if errorChan, isok := realTimeExchange.subChannelErrorMap[channel]; isok {
		return errorChan, nil
	} else {
		return nil, errors.Errorf("unknown channel %s", channel)
	}
}

func (realTimeExchange *RealTimeExchange) GetTradeChan(channel string) (chan []Trade, error) {
	if tradeChan, isok := realTimeExchange.tradeChanMap[channel]; isok {
		return tradeChan, nil
	} else {
		return nil, errors.Errorf("unknown channel %s", channel)
	}
}

func (realTimeExchange *RealTimeExchange) GetDepthChan(channel string) (chan *Depth, error) {
	if depthChan, isok := realTimeExchange.depthChanMap[channel]; isok {
		return depthChan, nil
	} else {
		return nil, errors.Errorf("unknown channel %s", channel)
	}
}

func (realTimeExchange *RealTimeExchange) WriteJSONMessage(msg interface{}) error {
	realTimeExchange.writeMsgChan <- msg
	if err := <-realTimeExchange.writeErrorChan; err != nil {
		return errors.Wrapf(err, "write to chan %v", msg)
	}
	return nil
}

func (realTimeExchange *RealTimeExchange) SubChannel(channel string) error {
	realTimeExchange.subChannelErrorMap[channel] = make(chan error)

	msg := realTimeExchange.exchange.GenSubMessage(channel)
	if err := realTimeExchange.WriteJSONMessage(msg); err != nil {
		return errors.Wrap(err, "write json message")
	}

	if err := <-realTimeExchange.subChannelErrorMap[channel]; err != nil {
		return errors.Wrapf(err, "sub %s channel failed", channel)
	}

	return nil
}

func (realTimeExchange *RealTimeExchange) ListenDepth(pair CurrencyPair, depth chan *Depth) error {
	channel := realTimeExchange.exchange.GenChannel(pair, DEPTH_CHANNEL)
	realTimeExchange.depthChanMap[channel] = depth
	realTimeExchange.channelTypeMap[channel] = DEPTH_CHANNEL

	return realTimeExchange.SubChannel(channel)
}

func (realTimeExchange *RealTimeExchange) ListenTrade(pair CurrencyPair, trade chan []Trade) error {
	channel := realTimeExchange.exchange.GenChannel(pair, TRADE_CHANNEL)
	realTimeExchange.tradeChanMap[channel] = trade
	realTimeExchange.channelTypeMap[channel] = TRADE_CHANNEL

	return realTimeExchange.SubChannel(channel)
}

func (realTimeExchange *RealTimeExchange) connectWebsocket() error {
	url := realTimeExchange.exchange.GetWebsocketURL()
	if c, _, err := websocket.DefaultDialer.Dial(url, nil); err != nil {
		return errors.Wrap(err, fmt.Sprintf("websocket dial %s", url))
	} else {
		realTimeExchange.c = c
	}
	return nil
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
		for {
			select {
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
					if _, isClose := err.(*websocket.CloseError); isClose == true {
						if err := realTimeExchange.connectWebsocket(); err != nil {
							log.Fatalf("reconnect websocket failed: %v\n", err)
						}
					}
					continue
				}
				msgChan <- msg
			}
		}
	}()

	if keepAliveHandler := realTimeExchange.exchange.GetKeepAliveHandler(); keepAliveHandler != nil {
		go keepAliveHandler(ctx, realTimeExchange)
	}

	if messageHandler := realTimeExchange.exchange.GetMessageHandler(); messageHandler != nil {
		go messageHandler(ctx, realTimeExchange, msgChan)
	}

	return nil
}
