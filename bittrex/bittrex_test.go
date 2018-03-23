package bittrex

import (
	"github.com/thbourlove/GoEx"
	"net/http"
	"testing"
)

var b = New(http.DefaultClient, "94", "11")

func TestBittrex_GetTicker(t *testing.T) {
	ticker, err := b.GetTicker(goex.BTC_USDT)
	t.Log("err=>", err)
	t.Log("ticker=>", ticker)
}

func TestBittrex_GetDepth(t *testing.T) {
	dep, err := b.GetDepth(1, goex.BTC_USDT)
	t.Log("err=>", err)
	t.Log("ask=>", dep.AskList)
	t.Log("bid=>", dep.BidList)
}

func Test_GetAcc(t *testing.T) {
	acc, err := b.GetAccount()
	t.Log("err=>", err)
	t.Log("acc=>", acc)
}

func Test_Sell(t *testing.T) {
	acc, err := b.LimitSell(`1`, `1`, goex.NEO_ETH)
	t.Log("err=>", err)
	t.Log("acc=>", acc)
}
