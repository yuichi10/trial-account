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
	"time"
)

//insert into orders (transport_allocate,rental_from,rental_to, item_id, user_id, day_price)
//insert into items(user_id, product_name, oneday_price, longday_price, deposit_price, delay_price) values (3, 'Nikon', 5000, 4000, 10000, 6000)
//select * from orders where 

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

	//予約できる日にち（利用日からの日数)
	CAN_BOOK_DAY_FROM_RENTAL_FROM = -4

	//予約できるマージンの日数
	BOOK_MARGIN_DAYS = 3
)

const (
	//返す値のステータス
	SUCCESS = 1
	FAILED_PROVISON_SALE=2
)

func TestDB(w http.ResponseWriter, r *http.Request) {
	/*r.ParseForm()
	db := dbase.OpenDB()
	fromStr := r.Form.Get(RENTAL_FROM)
	from := strTimeToTime(fromStr)
	preMarginDay := from.AddDate(0,0,-BOOK_MARGIN_DAYS)
	toStr := r.Form.Get(RENTAL_TO)
	to := strTimeToTime(toStr)
	postMarginDay := to.AddDate(0,0, BOOK_MARGIN_DAYS)
	//select * from orders where '2016-09-08'>rental_to order by rental_to desc limit 1
	//SELECT rental_to, order_id FROM orders WHERE '2016-09-15'<rental_to order by rental_to desc limit 1
	//select * from orders where '2016-09-13'<rental_from order by rental_from asc limit 5
	//from := strTimeToTime(fromStr)
	//一個前の日程を調べる
	dbWhere := fmt.Sprintf("'%v'>%v", timeToStrYMD(preMarginDay), RENTAL_TO)
	dbSql := fmt.Sprintf("SELECT %v, %v FROM %v WHERE %v ", RENTAL_TO, ORDER_ID, ORDER, dbWhere)
	fmt.Fprintf(w, "sql: %v ", dbSql)
	var near_rental_end time.Time
	var orderID int
	res, err := db.Query(dbSql)
	if err != nil {
		fmt.Fprintf(w, "query err: %v ", err)
		return
	}
	for res.Next() {
		if err := res.Scan(&near_rental_end, &orderID); err != nil {
			fmt.Fprintf(w, "scan err: %v ", err)
			return
		}
		fmt.Fprintf(w, "一番近いレンタル\norderID: %v\n最後の日: %v", orderID, near_rental_end)
	}*/
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
	//運営料金
	managePrice := 0
	if period := calcSubDate(rentalFrom, rentalTo); (period + 1) > 1 {
		dayPrice = iData.Longday_price
		amount = (period + 1) * dayPrice + managePrice
	} else {
		dayPrice = iData.Oneday_price
		amount = dayPrice + managePrice
	}
	fmt.Fprintf(w, "一日の料金は%v 合計料金は%v　です。\n", dayPrice, amount)
	//新しいオーダーの作成
	dbSql = "INSERT orders SET transport_allocate=?, rental_from=?, rental_to=?, item_id=?, user_id=?, day_price=?, management_charge=?, amount=?"
	stmt, _ := db.Prepare(dbSql)
	insRes, err := stmt.Exec(0, rFrom, rTo, itemID, userID, dayPrice, managePrice, amount)
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
	orderID := r.Form.Get(ORDER_ID)
	itemID := r.Form.Get(ITEM_ID)
	order, _ := getOrderInfo(orderID, db)
	if (itemID != strconv.Itoa(order.Item_id)) {
		fmt.Fprintf(w, "商品が間違っています")
		return
	}
	//予約できる期間を過ぎていたら同意できない
	if !checkRentalDayStart(order.Rental_from.(time.Time)) {
		fmt.Fprintf(w, "もうすでに予約できません")
		return 
	}
	//すでに他のリクエストで同意されていたらできない
	if !checkDoubleBooking(order.Rental_from.(time.Time), order.Rental_to.(time.Time), strconv.Itoa(order.Item_id), db) {
		fmt.Fprintln(w, "すでに予約されています")
		return
	}
	//オーダーのID
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
	//他にリクエストを送ってるものがあったらそれをキャンセルする
	cancelOtherBookings(order.Rental_from.(time.Time), order.Rental_to.(time.Time), strconv.Itoa(order.Item_id), db)
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
	//仮売上のリミットから借りれるかどうかの判断
	if able = checkRentalProvisonLimit(from); !able {
		return able
	}
	//もうすでにその期間借りられてないかどうかのチェック
	if able = checkDoubleBooking(from, to, itemID, db); !able {
		return able
	}
	//利用日から考えて利用できないかどうかのチェック
	if able = checkRentalDayStart(from); !able {
		return able
	}
	//前後のレンタルの日程を調べてマージンが足りなかった場合予約できないようにする
	return true
}

//レンタルがスタートする人予約できる日の制限をチェックする
func checkRentalDayStart(from time.Time) bool {
	nowDay := time.Now()
	nowDay = timeToTimeYMD(nowDay)
	canRentalDay := from.AddDate(0,0,CAN_BOOK_DAY_FROM_RENTAL_FROM)
	subTime := canRentalDay.Sub(nowDay)
	fmt.Printf(" today: %v\nrental from : %v\ncanRental: %v\nsubMinutes: %v\n\n", nowDay, from, canRentalDay, subTime.Hours())
	if subTime.Minutes() < 0 {
		return false
	}
	return true
}

//仮売上の日程からチェック
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
	marginFrom := tFrom.AddDate(0,0,-BOOK_MARGIN_DAYS)
	marginTo := tTo.AddDate(0,0,BOOK_MARGIN_DAYS)
	from := timeToStrYMD(marginFrom)
	to := timeToStrYMD(marginTo)
	fmt.Printf("from -> to : %v -> %v \n", from, to)
	//ステータスのsql
	dbState := fmt.Sprintf("(%v=%v)", ORDER_STATUS, STATUS_GET_CONSENT)
	dbWhereTime := fmt.Sprintf("('%v' BETWEEN %v AND %v OR '%v' BETWEEN %v AND %v)", from, RENTAL_FROM, RENTAL_TO, to, RENTAL_FROM, RENTAL_TO)
	dbSql := fmt.Sprintf("SELECT count(*) FROM %v WHERE (%v=%v AND %v) AND %v", ORDER, ITEM_ID, itemID, dbState, dbWhereTime)
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

func cancelOtherBookings(tFrom, tTo time.Time, itemID string, db *sql.DB) {
	r := new(http.Request)
	marginFrom := tFrom.AddDate(0,0,-BOOK_MARGIN_DAYS)
	marginTo := tTo.AddDate(0,0,BOOK_MARGIN_DAYS)
	from := timeToStrYMD(marginFrom)
	to := timeToStrYMD(marginTo)
	//ステータスのsql
	dbState := fmt.Sprintf("(%v=%v)", ORDER_STATUS, STATUS_GET_PROVISION_SALE)
	dbSql := fmt.Sprintf("SELECT %v FROM %v WHERE (%v=%v AND %v) AND ('%v' BETWEEN %v AND %v OR '%v' BETWEEN %v AND %v)", ORDER_ID, ORDER, ITEM_ID, itemID, dbState, from, RENTAL_FROM, RENTAL_TO, to, RENTAL_FROM, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err := db.Query(dbSql)
	var orderID int
	if err != nil {
		fmt.Println("sql error cancelOtherBookings \n")
		return
	}
	for res.Next() {
		if err := res.Scan(&orderID); err != nil {
			fmt.Println("sql scan error cancelOtherBookings \n")
			return
		}
		freeCancel(strconv.Itoa(orderID), db, r)
		updateOrderState(strconv.Itoa(orderID), STATUS_CANCEL, db)
	}

	//レンタルする間に他のレンタルがある場合
	dbSql = fmt.Sprintf("SELECT %v FROM %v WHERE (%v=%v AND %v) AND ('%v'<%v AND '%v'>%v)", ORDER_ID, ORDER, ITEM_ID, itemID, dbState, from, RENTAL_FROM, to, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err = db.Query(dbSql)
	if err != nil {
		fmt.Println("sql error cancelOtherBookings \n")
		return
	}
	for res.Next() {
		if err := res.Scan(&orderID); err != nil {
			return
		}
		freeCancel(strconv.Itoa(orderID), db, r)
		updateOrderState(strconv.Itoa(orderID), STATUS_CANCEL, db)
	}
}

