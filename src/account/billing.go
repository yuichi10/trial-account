package account

import (
	_ "database/sql"
	"dbase"
	_ "encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/http"
	_ "reflect"
	"strconv"
	"strings"
	"time"
)

//insert into orders (transport_allocate,rental_from,rental_to, item_id, user_id, day_price)
//insert into items(user_id, product_name, oneday_price, longday_price, deposit_price, delay_price) values (3, 'Nikon', 5000, 4000, 10000, 6000)

///orderに料金を設定する理由は後でプロダクトの値段が変わったらレシートの意味がないから
//orderのお金が仮売上→実売上のステータスを持っておくべき
//depositもお金をとったかどうかのステータスを持っておくべき

//キャンセル0 キャンセルなし
//キャンセル1　通常キャンセル
//キャンセル2 遅延によるキャンセル

//ステータス
//オーダーの同意がキャンセルもしくは仮売上が取得できなかった時-1
//オーダーが存在している0
//仮売上をとった1
//実売上をとった2
//デポジットをを考えている 3
//すべての工程が終了

const (
	//トークン
	TOKEN = "token"
	//ユーザーID
	USER_ID = "user_id"
	//プロダクトID
	ITEM_ID = "item_id"
	//オーダーID
	ORDER_ID = "order_id"
	//レンタルの開始日
	RENTAL_FROM = "rental_from"
	//レンタル終了日
	RENTAL_TO = "rental_to"
)

//利用日の次の日が44日目(仮売上期限の一日前)->利用日は仮売上期限の二日前
const DAY_LIMIT int = 43

//アイテムデータ
type itemData struct {
	item_id       int
	user_id       int
	product_name  string
	oneday_price  int
	longday_price int
	deposit_price int
	delay_price   int
}

func TestDB(w http.ResponseWriter, r *http.Request) {
	db := dbase.OpenDB()
	defer db.Close()
	dbSql := fmt.Sprintf("INSERT users SET %s=?, %s=?", dbase.USER_CUSTMER_ID, dbase.USER_NAME)
	stmt, err := db.Prepare(dbSql)
	if err != nil {
		fmt.Fprintf(w, "sql: %v\nプリペアエラーに失敗しました ERROR: %v", dbSql, err)
		return
	}
	_, err = stmt.Exec("cus_id", "cus_name")
	if err != nil {
		fmt.Fprintf(w, "sql: %v\nユーザー作成に失敗しました ERROR: %v", dbSql, err)
	}
}

/**
 * カスタマーの追加
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func AddCustomer(w http.ResponseWriter, r *http.Request) {
	db := dbase.OpenDB()
	defer db.Close()
	r.ParseForm()

	token := r.Form.Get(WP_TOKEN)
	rawjson, err := WebpayAddclient(token, r)
	if err != nil {
		return
	}
	js, _ := simplejson.NewJson([]byte(rawjson))

	if val := js.Get(WP_ERROR).Interface(); val != nil {
		fmt.Fprintf(w, "トークンにエラーがありました: %v\n メッセージ%v\n", val, js.Get(WP_ERROR).Get("message").MustString())
		return
	}
	cusID := js.Get(WP_CUS_ID).MustString()
	cusName := js.Get(WP_CUS_CARD).Get(WP_CUS_CARD_NAME).MustString()
	last4 := js.Get(WP_CUS_CARD).Get(WP_CUS_CURD_LAST4).MustString()
	//ユーザーの追加
	dbSql := fmt.Sprintf("INSERT users SET %s=?, %s=?", dbase.USER_CUSTMER_ID, dbase.USER_NAME)
	stmt, _ := db.Prepare(dbSql)
	_, err = stmt.Exec(cusID, cusName)
	if err == nil {
		fmt.Fprintf(w, "新しいユーザー %v が追加されました.\n　クレジットカードは**** **** %v で登録しました。\nCustomer ID は %v です\n", cusName, last4, cusID)
	} else {
		fmt.Fprintf(w, "ユーザー作成に失敗しました ERROR: %v", err)
	}
}

/**
 * オーダーを作成する
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func PublishOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	//借りる側のユーザーID
	userID := r.Form.Get(USER_ID)
	//借りるアイテム
	itemID := r.Form.Get(ITEM_ID)
	//レンタル期間
	rTo := r.Form.Get(RENTAL_TO)
	rFrom := r.Form.Get(RENTAL_FROM)
	//現在時刻
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//レンタル開始日
	y, m, d := divideTime(rFrom)
	rentalFrom := time.Date(y, time.Month(m), d, nowTime.Hour(), nowTime.Minute(), nowTime.Second(), 0, time.UTC)
	//レンタル終了日
	var rentalTo time.Time
	if rTo == "" {
		//最後の日が指定してなかった場合
		rentalTo = rentalFrom
		rTo = rFrom
	} else {
		y, m, d = divideTime(rTo)
		rentalTo = time.Date(y, time.Month(m), d, nowTime.Hour(), nowTime.Minute(), nowTime.Second(), 0, time.UTC)
	}
	//利用できる日かどうか
	if !canRentalDay(rentalFrom, rentalTo) {
		fmt.Fprintf(w, "%vから%vはレンタルできません\n", rentalFrom, rentalTo)
		return
	}

	//アイテムの情報を取得(料金などを設定するため)
	iData := new(itemData)
	dbSql := fmt.Sprintf("SELECT * FROM items where %v=%v", ITEM_ID, itemID)
	res, err := db.Query(dbSql)
	if err != nil {
		fmt.Fprintf(w, "%v \n select item ERR: %v\n", dbSql, err)
		return
	}
	for res.Next() {
		if err := res.Scan(&iData.item_id, &iData.user_id, &iData.product_name, &iData.oneday_price, &iData.longday_price, &iData.deposit_price, &iData.delay_price); err != nil {
			fmt.Fprintf(w, "scan item err: %v", err)
			return
		}
	}
	fmt.Fprintf(w, "プロダクトデータ: %v \n", iData)

	//料金を設定
	var dayPrice int
	var amount int
	if period := calcSubDate(rentalFrom, rentalTo); (period + 1) > 1 {
		dayPrice = iData.longday_price
		amount = (period + 1) * dayPrice
	} else {
		dayPrice = iData.oneday_price
		amount = dayPrice
	}
	fmt.Fprintf(w, "一日の料金は%v 合計料金は%v　です。\n", dayPrice, amount)
	//新しいオーダーの作成
	dbSql = "INSERT orders SET transport_allocate=?, rental_from=?, rental_to=?, item_id=?, user_id=?, day_price=?, amount=?"
	stmt, _ := db.Prepare(dbSql)
	insRes, err := stmt.Exec(0, rFrom, rTo, itemID, userID, dayPrice, amount)
	if err != nil {
		fmt.Fprintf(w, "オーダーのエラー: %v \n", err)
		return
	}
	orderLastID, _ := insRes.LastInsertId()
	fmt.Fprintf(w, "オーダを作りました\n プロダクトIDは%v\n オーダーのIDは%v\n", itemID, orderLastID)
	//仮売上を取得
	userID_int, _ := strconv.Atoi(userID)
	cID, _ := getCustomerID(userID_int)
	if err != nil {
		fmt.Fprintf(w, "customer id err: %v \n", err)
		return
	}
	fmt.Fprintf(w, "customer ID: %v\n", cID)
	wpcRawJson, _ := webpayCreateProvisionalSale(cID, strconv.Itoa(amount), r)
	jsJson, _ := simplejson.NewJson([]byte(wpcRawJson))
	//ウェブペイのIDを登録
	dbSql = "UPDATE orders SET order_charge_id=? where order_id=?"
	stmt, _ = db.Prepare(dbSql)
	_, err = stmt.Exec(jsJson.Get(WP_ID).MustString(), orderLastID)
	if err != nil {
		fmt.Fprintf(w, "アップデート: %v \n", err)
		return
	}
}

/**
 * オーダーに同意(かす側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func ConsentOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	//アイテムのID
	//itemID := r.Form.Get(ITEM_ID)
	//オーダーのID
	//orderID := r.Form.Get(ORDER_ID)
	//Orderのconsentをtrueに

	//同意されなかったら仮売上の削除
}

/**
 * オーダーのキャンセル(借りる側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func CanselOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	//ユーザーID
	//userID := r.Form.Get(USER_ID)
	//オーダーID
	//orderID := r.Form.Get(ORDER_ID)
	//キャンセルをtrueに
	//もし届け日の４日以内の場合キャンセル料分だけを実売上に
	//それより前のキャンセルの場合仮売上全部をキャンセル
}

/**
 * 借りる側が商品が遅れたことによるキャンセルまたは続行のレポート(利用日初日の24時間のみ)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func DelayCanselReport(w http.ResponseWriter, r *http.Request) {

}

/**
 * オーダーの仮売上を実売上に
 */
func ProvisionOrderToReal() {

}

/**
 * デポジットの作成
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func StartNegtiateDeposit(w http.ResponseWriter, r *http.Request) {
	//理由から遅延料金、保険料金を設定
}

/**
 * デポジット料金をアップデート
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func UploadDeposit(w http.ResponseWriter, r *http.Request) {

}

/**
 * デポジット料金の同意(両方)
 * @param  {[type]} w http.ResponseWriter [description]
 * @param  {[type]} r *http.Request       [description]
 * @return {[type]}   [description]
 */
func consentDeposit(w http.ResponseWriter, r *http.Request) {
}

func getCustomerID(userID int) (string, error) {
	db := dbase.OpenDB()
	defer db.Close()
	dbSql := fmt.Sprintf("SELECT credit_customer_id FROM users where user_id=%v", userID)
	var customerID string
	res, err := db.Query(dbSql)
	if err != nil {
		return "", err
	}
	for res.Next() {
		if err := res.Scan(&customerID); err != nil {
			return "", err
		}
	}
	return customerID, nil
}

func canRentalDay(pre, post time.Time) bool {
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//契約できる日かどうか(今はとりあえず仮売上の日にちを超えないようになってるかどうか)
	subDays := calcSubDate(nowTime, pre)
	if subDays > DAY_LIMIT {
		return false
	}
	return true
	//それ以外のオーダーでその日に被ってないかどうか
}

/**
 * 2016-02-12 の形を崩す
 * @param  {[type]} allTime string)       (y, m, d int [description]
 * @return {[type]}         [description]
 */
func divideTime(allTime string) (y, m, d int) {
	divTime := strings.Split(allTime, "-")
	y, _ = strconv.Atoi(divTime[0])
	m, _ = strconv.Atoi(divTime[1])
	d, _ = strconv.Atoi(divTime[2])
	return
}

/**
 * 借りる日数を計算
 */
func calcSubDate(pre, post time.Time) int {
	subTime := post.Sub(pre)
	return int(subTime.Hours() / 24)
}
