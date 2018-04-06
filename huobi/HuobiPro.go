package huobi

import "net/http"

type HuobiPro struct {
	*HuoBi_V2
}

func NewHuobiPro(client *http.Client, apikey, secretkey, accountId string) *HuobiPro {
	hbv2 := NewV2(client, apikey, secretkey, accountId)
	hbv2.baseUrl = "https://api.huobi.pro"
	return &HuobiPro{hbv2}
}

func (hbpro *HuobiPro) GetExchangeName() string {
	return "huobi.pro"
}
