package kucoin

import (
	"github.com/nntaoli-project/GoEx"
	"net/http"
	"testing"
)

var ku = New(http.DefaultClient, "111", "03f3a5dc")

func Test_GetDepth(t *testing.T) {
	dep, err := ku.GetDepth(1, goex.BTC_USDT)
	t.Log("err=>", err)
	t.Log("ask=>", dep.AskList)
	t.Log("bid=>", dep.BidList)
}

func Test_GetAcc(t *testing.T) {
	ku.baseUrl = `https://private-anon-ffe74721e9-kucoinapidocs.apiary-mock.com`
	acc, err := ku.GetAccount()
	t.Log("err=>", err)
	t.Log("acc=>", acc)
}

func Test_Sell(t *testing.T) {
	ku.baseUrl = `https://private-anon-ffe74721e9-kucoinapidocs.apiary-mock.com`
	order, err := ku.LimitSell(`1`, `1`, goex.NEO_ETH)
	t.Log("err=>", err)
	t.Log("acc=>", order)
}
