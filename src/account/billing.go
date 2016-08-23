package account

import (
	"database/sql"
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
)

func TestDB(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	orderID := r.Form.Get(ORDER_ID)
	checkDepositLimit(orderID, db)
	/*
		db, err := dbase.OpenDbr()
		if err != nil {
			fmt.Fprintf(w, "オープンERR: %v \n", err)
		}
		tx, err := db.Begin()
		if err != nil {
			fmt.Fprintf(w, "トランザクションエラー: %v \n", err)
			return
		}
		_, err = db.InsertInto("items").
			Columns("user_id", "product_name", "oneday_price", "longday_price", "deposit_price", "delay_price").
			Values(3, "Canon E 10", 4800, 4200, 12000, 7000).
			Exec()
		if err != nil {
			tx.Rollback()
			return
		} else {

			err := tx.Rollback()
			fmt.Fprintf(w, "できたけどロールバック: %v \n", err)
			return
		}
		var ot []orderType

		db.Select("*").From("items").Load(&ot)
		fmt.Fprintf(w, "製品一覧 %v", ot[0])
		tx.Commit()*/
	//db := dbase.OpenDB()
	//defer db.Close()
	//tx, err := db.Begin()
	//if err != nil {
	//	fmt.Fprintf(w, "トランザクションエラー")
	//	return
	//}
	//dbSql := "INSERT INTO items (user_id, product_name, oneday_price, longday_price, deposit_price, delay_price) VALUES(?, ?, ?, ?, ?, ?)"
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
	userID := r.Form.Get(USER_ID)
	itemID := r.Form.Get(ITEM_ID)
	//レンタル期間
	rTo := r.Form.Get(RENTAL_TO)
	rFrom := r.Form.Get(RENTAL_FROM)
	//現在時刻
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//レンタル開始日
	rentalFrom := strTimeToTime(rFrom)
	//レンタル終了日
	var rentalTo time.Time
	if rTo == "" {
		//最後の日が指定してなかった場合
		rentalTo = rentalFrom
		rTo = rFrom
	} else {
		rentalTo = strTimeToTime(rTo)
	}
	//利用できる日かどうか
	if !checkRentalDay(rentalFrom, rentalTo, itemID, db) {
		fmt.Fprintf(w, "%vから%vはレンタルできません\n", rentalFrom, rentalTo)
		return
	}
	var dbSql string
	//アイテムの情報を取得(料金などを設定するため)
	iData, _ := getItemData(itemID, db)
	fmt.Fprintf(w, "プロダクトデータ: %v \n", iData)

	//料金を設定
	var dayPrice int
	var amount int
	if period := calcSubDate(rentalFrom, rentalTo); (period + 1) > 1 {
		dayPrice = iData.Longday_price
		amount = (period + 1) * dayPrice
	} else {
		dayPrice = iData.Oneday_price
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
	cID, _ := getCustomerID(userID_int, db)
	if err != nil {
		fmt.Fprintf(w, "customer id err: %v \n", err)
		return
	}
	fmt.Fprintf(w, "customer ID: %v\n", cID)
	wpcRawJson, _ := webpayCreateProvisionalSale(cID, strconv.Itoa(amount), r)
	jsJson, _ := simplejson.NewJson([]byte(wpcRawJson))
	if is, mes := checkCardError(jsJson); !is {
		fmt.Fprintf(w, "エラーメッセージ: %v \n", mes)
		//ステータスを変更
		if err := updateOrderState(strconv.Itoa(int(orderLastID)), STATUS_FAILED_PROVISION_SALE, db); err != nil {
			fmt.Fprintf(w, "update State error: %v \n ", err)
			return
		}
	} else {
		//ウェブペイのIDを登録とすてーたすの変更
		dbSql = "UPDATE orders SET order_charge_id=?, status=? where order_id=?"
		stmt, _ = db.Prepare(dbSql)
		_, err = stmt.Exec(jsJson.Get(WP_ID).MustString(), STATUS_GET_PROVISION_SALE, orderLastID)
		if err != nil {
			fmt.Fprintf(w, "アップデート: %v \n", err)
			return
		}
	}
}

/**
 * オーダーに同意(かす側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func ConsentOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	//オーダーのID
	orderID := r.Form.Get(ORDER_ID)
	if checkOrderStatus(orderID, db, []int{STATUS_GET_CONSENT}...) {
		fmt.Fprintf(w, "すでに同意されています \n")
		return
	}
	if chState := checkOrderStatus(orderID, db, []int{STATUS_GET_PROVISION_SALE}...); !chState {
		fmt.Fprintf(w, "ステータスの問題で同意できません status: チェック: %v \n", chState)
		return
	}
	//Orderのconsentをtrueに
	if err := updateOrderState(orderID, STATUS_GET_CONSENT, db); err != nil {
		fmt.Fprintf(w, "同意できませんでした\n")
		return
	}
	fmt.Fprintf(w, "同意されました。\n")
}

/**
 * オーダーが同意しなかった時
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func DisagreeOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	orderID := r.Form.Get(ORDER_ID)
	if chState := checkOrderStatus(orderID, db, []int{STATUS_GET_PROVISION_SALE}...); !chState {
		fmt.Fprintf(w, "ステータスの問題で非同意ができません status: チェック: %v \n", chState)
		return
	}
	if err := updateOrderState(orderID, STATUS_FAILED_CONSENT, db); err != nil {
		fmt.Fprintf(w, "非同意ができませんでした\n")
		return
	}
	fmt.Fprintf(w, "同意がキャンセルされました\n")
	chargeID, err := getChargeID(orderID, db)
	if err != nil {
		fmt.Fprintf(w, "charge ID ERR: %v\n", err)
	}
	//仮売上をキャンセル
	rawjson, err := webpayCancelProvisionalSale(chargeID, r)
	if err != nil {
		if err := updateOrderState(orderID, STATUS_FAILED_CONSENT_PAY_BACK, db); err != nil {
			fmt.Fprintf(w, "ステータスの変更の失敗", err)
		}
	}
	js, _ := simplejson.NewJson([]byte(rawjson))
	if ok, msg := checkCardError(js); !ok {
		fmt.Fprintf(w, "カードにエラー: %v", msg)
		if err := updateOrderState(orderID, STATUS_FAILED_CONSENT_PAY_BACK, db); err != nil {
			fmt.Fprintf(w, "ステータスの変更の失敗: %v", err)
			return
		}
		return
	}
	fmt.Fprintf(w, "rawjson: %v \n err: %v \n", rawjson, err)
}

/**
 * オーダーの仮売上を実売上に
 */
func ProvisionOrderToReal() {
	//
}

/**
 * リクエストで送られたデータのチェック
 * @return {[type]} [description]
 */
func checkRequestData() {
	//
}

/**
 * ステータスのチェックstateが一致しているものがあったらtrueを返す
 * @return {[type]} [description]
 */
func checkOrderStatus(orderID string, db *sql.DB, status ...int) bool {
	orderState, err := getOrderStatus(orderID, db)
	if err != nil {
		fmt.Printf("チェックステータスエラー: %v \n", err)
		return false
	}
	for _, state := range status {
		if state == orderState {
			return true
		}
	}
	return false
}

/**
 * かせるかどうかの日にち判定
 */
func checkRentalDay(from, to time.Time, itemID string, db *sql.DB) bool {
	var able bool
	if able = checkRentalProvisonLimit(from); !able {
		return able
	}

	if able = checkDoubleBooking(from, to, itemID, db); !able {
		return able
	}
	return true
}

func checkRentalDayStart() {
	//今日より過去の場合もしくは開始日の一日前より以前は予約できないようにする
}

func checkRentalProvisonLimit(from time.Time) bool {
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//契約できる日かどうか(今はとりあえず仮売上の日にちを超えないようになってるかどうか)
	subDays := calcSubDate(nowTime, from)
	fmt.Printf("%v : %v 時間差: %v \n", from, nowTime, subDays)
	if subDays > DAY_LIMIT {
		return false
	}
	return true
}

//その日にもう借りられてないかどうか
func checkDoubleBooking(tFrom, tTo time.Time, itemID string, db *sql.DB) bool {
	//始まりか終わりどちらかが利用期間にかかってる
	//SELECT count(*) FROM orders WHERE (item_id=4 AND (status=1 OR status=2)) AND ('2016-8-22' BETWEEN rental_from AND rental_to OR '2016-8-22' BETWEEN rental_from AND rental_to);
	var count int = 0
	from := timeToStrYMD(tFrom)
	to := timeToStrYMD(tTo)
	//ステイトの部分のsql
	dbState := fmt.Sprintf("(%v=%v OR %v=%v)", ORDER_STATUS, STATUS_GET_PROVISION_SALE, ORDER_STATUS, STATUS_GET_CONSENT)
	dbSql := fmt.Sprintf("SELECT count(*) FROM %v WHERE (%v=%v AND %v) AND ('%v' BETWEEN %v AND %v OR '%v' BETWEEN %v AND %v)", ORDER, ITEM_ID, itemID, dbState, from, RENTAL_FROM, RENTAL_TO, to, RENTAL_FROM, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err := db.Query(dbSql)
	var count1 int
	if err != nil {
		return false
	}
	for res.Next() {
		if err := res.Scan(&count1); err != nil {
			return false
		}
	}
	count += count1
	//レンタルする間に他のレンタルがある場合
	dbSql = fmt.Sprintf("SELECT count(*) FROM %v WHERE (%v=%v AND %v) AND ('%v'<%v AND '%v'>%v)", ORDER, ITEM_ID, itemID, dbState, from, RENTAL_FROM, to, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err = db.Query(dbSql)
	var count2 int
	if err != nil {
		return false
	}
	for res.Next() {
		if err := res.Scan(&count2); err != nil {
			return false
		}
	}
	count += count2
	if count == 0 {
		return true
	}
	return false
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

func strTimeToTime(strTime string) time.Time {
	y, m, d := divideTime(strTime)
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

//time型から y-m-dの形に治す
func timeToStrYMD(t time.Time) string {
	y := t.Year()
	m := int(t.Month())
	d := t.Day()
	day := fmt.Sprintf("%v-%v-%v", y, m, d)
	return day
}

func timeToTimeYMD(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

/**
 * 借りる日数を計算
 */
func calcSubDate(pre, post time.Time) int {
	subTime := post.Sub(pre)
	days := int(subTime.Hours()) / 24
	if subTime.Minutes()/(24*60)-float64(days) > 0 {
		days += 1
	}
	return days
}
