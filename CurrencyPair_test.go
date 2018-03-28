package goex

import (
	"strings"
	"testing"
)

func TestCurrency2_String(t *testing.T) {
	btc := NewCurrency("btc")
	btc2 := Currency{"BTC" }
	t.Log(btc == BTC)
	t.Log(btc == btc2)
}

func TestCurrencyPair2_String(t *testing.T) {
	btccny := NewCurrencyPair(NewCurrency("btc"), NewCurrency("cny"))
	t.Log(strings.ToUpper(btccny.String()))
	t.Log(BTC_CNY)
}
