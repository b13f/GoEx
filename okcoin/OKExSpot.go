package okcoin

import (
	"encoding/json"
	"fmt"
	. "github.com/nntaoli-project/GoEx"
	"net/http"
	"net/url"
	"strings"
)

type OKExSpot struct {
	OKCoinCN_API
}

func NewOKExSpot(client *http.Client, accesskey, secretkey string) *OKExSpot {
	return &OKExSpot{
		OKCoinCN_API{client, accesskey, secretkey, "https://www.okex.com/api/v1/"}}
}

func (ctx *OKExSpot) GetExchangeName() string {
	return "okex.com"
}

func (ctx *OKExSpot) GetAccount() (*Account, error) {
	postData := url.Values{}
	err := ctx.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	body, err := HttpPostForm(ctx.client, ctx.api_base_url+url_userinfo, postData)
	if err != nil {
		return nil, err
	}

	resp := struct {
		Info struct {
			Funds struct {
				Free    map[string]string `json:"free"`
				Freezed map[string]string `json:"freezed"`
			} `json:"funds"`
		} `json:"info"`
		Result bool    `json:"result"`
		Error  float64 `json:"error_code"`
	}{}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Error > 0 {
		return nil, fmt.Errorf("%f", resp.Error)
	}

	acc := new(Account)
	acc.Exchange = ctx.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	for curr, amount := range resp.Info.Funds.Free {
		if ToFloat64(amount) == 0 {
			continue
		}
		currency := NewCurrency(strings.ToUpper(curr), "")
		acc.SubAccounts[currency] = SubAccount{
			Currency: currency,
			Amount:   ToFloat64(amount),
		}
	}

	for curr, frozenAmount := range resp.Info.Funds.Freezed {
		if ToFloat64(frozenAmount) == 0 {
			continue
		}

		currency := NewCurrency(strings.ToUpper(curr), "")
		if t, ok := acc.SubAccounts[currency]; ok {
			t.ForzenAmount = ToFloat64(frozenAmount)
			acc.SubAccounts[currency] = t
		} else {
			acc.SubAccounts[currency] = SubAccount{
				Currency:     currency,
				ForzenAmount: ToFloat64(frozenAmount),
			}
		}
	}

	return acc, nil
}
