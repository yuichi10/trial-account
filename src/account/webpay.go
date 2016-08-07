package account

import (
	//"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	PRIVATE_KEY    = "test_secret_5QM6V828E0OveQocUQ8uO32R"
	CONNECT_POST   = "POST"
	CONNECT_GET    = "GET"
	CONNECT_DELETE = "DELETE"
	AMOUNT         = "amount"

	CUSTOMER_TOKEN = "customerToken"
	CARD_TOKEN     = "cardToken"
	CHARGE_TOKEN   = "chargeToken"
)

/*
自身で設定
*/
func webpayConnect(rawurl string, data url.Values, r *http.Request, connectType string) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest(
		connectType,
		rawurl,
		strings.NewReader(data.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(PRIVATE_KEY, "")
	resp, err1 := client.Do(req)
	if err1 != nil {
		return "client.Do: ", err1
	}
	defer resp.Body.Close()
	body, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		return "ReadAll: ", err2
	}
	return string(body), nil
}

/**
 * チャージ
 * @param  {[type]} w http.ResponseWriter [description]
 * @param  {[type]} r *http.Request       [description]
 * @return {[type]}   [description]
 */
func webpayCharge(w http.ResponseWriter, r *http.Request) {
	/*
		定義POST https://api.webpay.jp/v1/chargesリクエスト例$curl "https://api.webpay.jp/v1/charges" \
		-u "test_secret_5QM6V828E0OveQocUQ8uO32R": \
		-d "amount=400" \
		-d "currency=jpy" \
		-d "card=tok_SampleCARD_TOKEN" \
		-d "description=アイテムの購入"
	*/
	r.ParseForm()
	token := r.Form.Get(CARD_TOKEN)

	rawurl := "https://api.webpay.jp/v1/charges"
	data := url.Values{
		"amount":   {"400"},
		"currency": {"jpy"},
		"card":     {token},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}

/*
	カスタマーidを用いた課金
*/
func webpayChargeCustomer(w http.ResponseWriter, r *http.Request) {
	/*
		定義POST https://api.webpay.jp/v1/chargesリクエスト例$curl "https://api.webpay.jp/v1/charges" \
		-u "test_secret_5QM6V828E0OveQocUQ8uO32R": \
		-d "amount=400" \
		-d "currency=jpy" \
		-d "card=tok_SampleCardToken" \
		-d "description=アイテムの購入"
	*/
	r.ParseForm()
	//トークン
	token := r.Form.Get(CUSTOMER_TOKEN)

	rawurl := "https://api.webpay.jp/v1/charges"
	data := url.Values{
		"amount":   {"300"},
		"currency": {"jpy"},
		"customer": {token},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}

/*
	クライアントを追加
*/
func webpayAddclient(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token := r.Form.Get(CARD_TOKEN)
	rawurl := "https://api.webpay.jp/v1/customers"
	data := url.Values{
		"card": {token},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}
func WebpayAddclient(token string, r *http.Request) (string, error) {
	rawurl := "https://api.webpay.jp/v1/customers"
	data := url.Values{
		"card": {token},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	return rawjson, err
}

/*
	クライアントの情報を取得
*/
func webpayGetClientsInfo(w http.ResponseWriter, r *http.Request) {
	//クライアントID
	r.ParseForm()
	token := r.Form.Get(CUSTOMER_TOKEN)
	rawurl := "https://api.webpay.jp/v1/customers/" + token
	data := url.Values{}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_GET)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}

/**
 * 課金の払い戻し
 * @param  {[type]} w http.ResponseWriter [description]
 * @param  {[type]} r *http.Request       [description]
 * @return {[type]}   [description]
 */
func webpayChargeRollback(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	//チャージID
	token := r.Form.Get(CHARGE_TOKEN)
	rawurl := "https://api.webpay.jp/v1/charges/" + token + "/refund"
	data := url.Values{}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}

/*
	定期課金
*/
func webpayRecursion(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token := r.Form.Get(CUSTOMER_TOKEN)
	rawurl := "https://api.webpay.jp/v1/recursions"
	data := url.Values{
		"amount":      {"400"},
		"currency":    {"jpy"},
		"customer":    {token},
		"period":      {"month"},
		"description": {"テスト課金月額"},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}

/*
	仮売上の作成
*/
func webpayProvisionalSale(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token := r.Form.Get(CUSTOMER_TOKEN)
	amount := r.Form.Get(AMOUNT)
	rawurl := "https://api.webpay.jp/v1/charges"
	data := url.Values{
		"amount":      {amount},
		"currency":    {"jpy"},
		"customer":    {token},
		"capture":     {"false"},
		"expire_days": {"45"},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v amount: %v", rawjson, err, amount)
}
func webpayCreateProvisionalSale(customerToken string, amount string, r *http.Request) (string, error) {
	rawurl := "https://api.webpay.jp/v1/charges"
	data := url.Values{
		"amount":      {amount},
		"currency":    {"jpy"},
		"customer":    {customerToken},
		"capture":     {"false"},
		"expire_days": {"45"},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	return rawjson, err
}

/*
	仮売上の無効化
*/
func webpayProvisionalSaleCancel(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token := r.Form.Get(CHARGE_TOKEN)
	rawurl := "https://api.webpay.jp/v1/charges/" + token + "/refund"
	data := url.Values{}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "rawurl: %v \n error: %v \n", rawjson, err)
}
func webpayCancelProvisionalSale(ch_token string, r *http.Request) (string, error) {
	rawurl := "https://api.webpay.jp/v1/charges/" + ch_token + "/refund"
	data := url.Values{}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	return rawjson, err
}

/*
	仮売上を実売上に変更
*/
func webpayProvisionalSaleToReal(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token := r.Form.Get(CHARGE_TOKEN)
	amount := r.Form.Get(AMOUNT)
	rawurl := "https://api.webpay.jp/v1/charges/" + token + "/capture"
	data := url.Values{
		"amount": {amount},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	fmt.Fprintf(w, "%v %v", rawjson, err)
}
func webpayProvisionalToReal(chargeId string, amount string, r *http.Request) string {
	rawurl := "https://api.webpay.jp/v1/charges/" + chargeId + "/capture"
	data := url.Values{
		"amount": {amount},
	}
	rawjson, err := webpayConnect(rawurl, data, r, CONNECT_POST)
	_ = err
	return rawjson
}
