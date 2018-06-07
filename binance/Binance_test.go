package binance

import (
	"testing"
	"net/http"
	"fmt"
	"github.com/nntaoli-project/GoEx"
	"time"
)
//IARB_APIKEY=HCGX6hjsdCOiIfhNyyHWo3fHyOWVu5lVVINq0wGLtvN1maUOp20CcojfkHRDIzU2 \
//IARB_APISECRET=jGxshslr5OwKh014aQ5uDEWIs1yjZhr7TUUb0oRjBqA7v84w2wnjfA8t6LJs1Xy2
var ba = New(http.DefaultClient, "HCGX6hjsdCOiIfhNyyHWo3fHyOWVu5lVVINq0wGLtvN1maUOp20CcojfkHRDIzU2", "jGxshslr5OwKh014aQ5uDEWIs1yjZhr7TUUb0oRjBqA7v84w2wnjfA8t6LJs1Xy2")
//
//func TestBinance_GetTicker(t *testing.T) {
//	ticker, _ := ba.GetTicker(goex.LTC_BTC)
//	t.Log(ticker)
//}
//func TestBinance_LimitSell(t *testing.T) {
//	order, err := ba.LimitSell("1", "1", goex.LTC_BTC)
//	t.Log(order, err)
//}
//
//func TestBinance_GetDepth(t *testing.T) {
//	dep, err := ba.GetDepth(5, goex.ETH_BTC)
//	t.Log(err)
//	if err == nil {
//		t.Log(dep.AskList)
//		t.Log(dep.BidList)
//	}
//}
//
//func TestBinance_GetAccount(t *testing.T) {
//	account, err := ba.GetAccount()
//	t.Log(account, err)
//}
//
//func TestBinance_GetUnfinishOrders(t *testing.T) {
//	orders, err := ba.GetUnfinishOrders(goex.ETH_BTC)
//	t.Log(orders, err)
//}

func TestOrders(t *testing.T)  {

	ch,_ := ba.GetOrdersChan()

	go func() {
		for {
			time.Sleep(2*time.Second)
			order, err := ba.LimitSell("1", "1", goex.NEO_BTC)
			if err!=nil {
				fmt.Printf("place order error %s\n",err)
			} else {
				fmt.Printf("place order id %s\n", order.Id)

				time.Sleep(2*time.Second)
				ba.CancelOrder(order.Id, goex.NEO_BTC)
				fmt.Printf("order canceled %s\n", order.Id)
			}

		}
	}()

	//	t.Log(order, err)
	for o := range ch {
		fmt.Printf("res %+v\n",o)
	}
}