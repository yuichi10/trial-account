package account

import (
	"database/sql"
	"dbase"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/http"
	_ "reflect"
	"strconv"
	"time"
)

const (
	//トークン
	TOKEN = "token"
	//予約できる日にち（利用日からの日数)
	CAN_BOOK_DAY_FROM_RENTAL_FROM = -4

	//予約できるマージンの日数
	BOOK_MARGIN_DAYS = 3
	MANAGEMENT_PRICE_RATE = 0
	INSURANCE_PRICE_RATE = 0
)

type resPropriety struct {
	IsSuccess bool `json:"isSuccess"`
	Error string `json:"error"`
}

var internalErrorJson string = "{\"error\":\"internal error\"}"

func TestDB(w http.ResponseWriter, r *http.Request) {
	//テスト
	db := dbase.OpenDB()
	defer db.Close()
	fromTime, toTime := getInputAjustedTimes("2013-03-10", "2013-03-15")
	order := new(orderType)
	order.Transport_allocate = 0
	order.Rental_from = fromTime
	order.Rental_to = toTime
	order.Item_id = 4
	order.User_id = 4
	order.Day_price = 5000
	order.After_day_price = 4000
	order.Insurance_price = 0
	order.Management_charge = 0
	order.Amount = 16000
	order.Cancel_price = 8000
	order.Cancel_date = nil
	order.Cancel_status = 0
	order.Status = 0
	order.insertOrderInfo(db)
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
	dbSql := fmt.Sprintf("INSERT users SET %s=?, %s=?",  USER_CREDIT_ID, USER_NAME)
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
	order := new(orderType)
	//ユーザーのIDのセット
	if !isExitInDBUnique(USER, USER_ID, userID, db) {
		proprietyResponse(false, "ユーザーIDが間違ってます", w)
		return
	}
	order.User_id, _ = strconv.Atoi(userID)
	//アイテムIDのセット
	itemID := r.Form.Get(ITEM_ID)
	if !isExitInDBUnique(ITEM, ITEM_ID, itemID, db) {
		proprietyResponse(false, "アイテムIDが間違ってます", w)
		return
	}
	order.Item_id, _ = strconv.Atoi(itemID)
	//レンタル期間のセット
	rTo := r.Form.Get(RENTAL_TO)
	if !checkStrTime(rTo) && rTo != ""{
		//レンタル終了日のチェック
		proprietyResponse(false, "レンタル終了日が間違ってます", w)
		return
	}
	rFrom := r.Form.Get(RENTAL_FROM)
	if !checkStrTime(rFrom) {
		//レンタル開始日のチェック
		proprietyResponse(false, "レンタル開始日が間違ってます", w)
		return
	}
	rentalFrom, rentalTo := getInputAjustedTimes(rFrom, rTo)
	//利用できる日かどうか
	if !checkRentalDay(rentalFrom, rentalTo, itemID, db) {
		proprietyResponse(false, "利用できる日ではありません", w)
		return
	}
	order.Rental_from = rentalFrom
	order.Rental_to = rentalTo
	//アイテムの情報を取得(料金などを設定するため)
	order.setOrderPrice(db)
	_, err := order.insertOrderInfo(db)
	//orderLastID, _ := res.LastInsertId()
	fmt.Fprintf(w, "オーダを作りました\n プロダクトIDは%v\n オーダーのIDは%v\n", itemID, order.Order_id)
	if err != nil {
		fmt.Printf("インサートオーダーエラー")
		proprietyResponse(false, "登録に失敗", w)
		return
	}
	//仮売上を取得
	state, err := order.getProvisonalSale(db, r)
	if err != nil && state == STATUS_FAILED_PROVISION_SALE {
		fmt.Printf("仮売上エラー err: %v\n", err)
		proprietyResponse(false, "仮売上の取得に失敗", w)
		return
	} 
	proprietyResponse(true, "", w)
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
	if itemID != strconv.Itoa(order.Item_id) {
		proprietyResponse(false, "商品が間違っています", w)
		return
	}
	//予約できる期間を過ぎていたら同意できない
	if !checkRentalDayStart(order.Rental_from.(time.Time)) {
		proprietyResponse(false, "もう予約できる期間を過ぎました", w)
		return 
	}
	//すでに他のリクエストで同意されていたらできない
	if !checkDoubleBooking(order.Rental_from.(time.Time), order.Rental_to.(time.Time), strconv.Itoa(order.Item_id), db) {
		proprietyResponse(false, "すでに予約されています", w)
		return
	}
	//オーダーのID
	if checkOrderStatus(orderID, db, []int{STATUS_GET_CONSENT}...) {
		proprietyResponse(false, "すでに同意されています", w)
		return
	}
	if chState := checkOrderStatus(orderID, db, []int{STATUS_GET_PROVISION_SALE}...); !chState {
		proprietyResponse(false, "ステータスの問題で同意できません", w)
		return
	}
	//Orderのconsentをtrueに
	if err := updateOrderState(orderID, STATUS_GET_CONSENT, db); err != nil {
		responseInternalError(w)
		return
	}
	//他にリクエストを送ってるものがあったらそれをキャンセルする
	err := cancelOtherBookings(order.Rental_from.(time.Time), order.Rental_to.(time.Time), strconv.Itoa(order.Item_id), db)
	if err != nil {
		responseInternalError(w)
		return
	}
	proprietyResponse(true, "", w)
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

func getInputAjustedTimes(from, to string) (rentalFrom, rentalTo time.Time) {
	//現在時刻
	//nowTime = time.Now()
	//nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//レンタル開始日
	rentalFrom, _ = strTimeToTime(from)
	if to == "" {
		//最後の日が指定してなかった場合
		rentalTo = rentalFrom
		to = from
	} else {
		rentalTo, _ = strTimeToTime(to)
	}
	return
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

func cancelOtherBookings(tFrom, tTo time.Time, itemID string, db *sql.DB) error {
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
		return err
	}
	for res.Next() {
		if err := res.Scan(&orderID); err != nil {
			return err
		}
		freeCancel(strconv.Itoa(orderID), db, r)
		updateOrderState(strconv.Itoa(orderID), STATUS_CANCEL, db)
	}

	//レンタルする間に他のレンタルがある場合
	dbSql = fmt.Sprintf("SELECT %v FROM %v WHERE (%v=%v AND %v) AND ('%v'<%v AND '%v'>%v)", ORDER_ID, ORDER, ITEM_ID, itemID, dbState, from, RENTAL_FROM, to, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err = db.Query(dbSql)
	if err != nil {
		return err
	}
	for res.Next() {
		if err := res.Scan(&orderID); err != nil {
			return err
		}
		freeCancel(strconv.Itoa(orderID), db, r)
		updateOrderState(strconv.Itoa(orderID), STATUS_CANCEL, db)
	}
	return nil
}


func (res *resPropriety) setResPropriety(success bool, errorMsg string)  {
	res.IsSuccess = success
	res.Error = errorMsg
}

//手数料の計算
func calcManagementCharge(usagePrice int) int {
	return usagePrice * MANAGEMENT_PRICE_RATE
}

//保険金の計算
func calcInsurancePrice(dayPrice int) int {
	return dayPrice * INSURANCE_PRICE_RATE
}

//sqlなどのサーバーのエラーが起こった時に返す値
func responseInternalError(w http.ResponseWriter) {
	w = setResponseJsonHeader(http.StatusInternalServerError, w)
	fmt.Fprintf(w, internalErrorJson)
}

//条件に一致しなかった時にproprietyを返す
func proprietyResponse(success bool, errMsg string, w http.ResponseWriter) {
	response := new(resPropriety)
	response.setResPropriety(success, errMsg)
	res, err := json.Marshal(response)
	if err != nil {
		w = setResponseJsonHeader(http.StatusInternalServerError, w)
		fmt.Fprintf(w, internalErrorJson)
		return
	}
	w = setResponseJsonHeader(http.StatusOK, w)
	fmt.Fprintf(w, string(res))
}

//jsonを返す時のheaderを書く
func setResponseJsonHeader(state int, w http.ResponseWriter) http.ResponseWriter {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(state)
	return w
}
