package account

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

const (
	//オーダー
	ORDER = "orders"
	//オーダーID
	ORDER_ID = "order_id"
	//レンタルの開始日
	RENTAL_FROM = "rental_from"
	//レンタル終了日
	RENTAL_TO          = "rental_to"
	ORDER_CHARGE_ID    = "order_charge_id"
	ORDER_CONSENT      = "order_consent"
	ORDER_STATUS       = "status"
	ORDER_CANCEL_DATE  = "cancel_date"
	ORDER_CANCEL_STATE = "cancel_status"
	ORDER_AMOUNT       = "amount"
	IS_CANCEL          = "is_cancel"
)

const (
	//オーダーのキャンセルのステータス
	STATUS_FAILED_DELAY_CANCEL = -3 //遅延によるキャンセルの失敗
	STATUS_FAILED_CANCEL_PAY   = -2 //有料キャンセルの失敗
	STATUS_FAILED_CANCEL_FREE  = -1 //無料キャンセルの失敗
	ORDER_STATE_CANCEL_NONE    = 0  //キャンセル無し
	ORDER_STATE_CANCEL_FREE    = 1  //無料のキャンセル
	ORDER_STATE_CANCEL_PAID    = 2  //有料のキャンセル
	ORDER_STATE_CANCEL_DELAY   = 3  //遅延キャンセル
)

const (
	//オーダーステータス
	STATUS_CONTINUE_DELAY_FAILED        = -5
	STATUS_FAILED_REAL_SALE             = -4 //実売上の取得に失敗
	STATUS_FAILED_CONSENT_PAY_BACK      = -3 //オーダーがキャンセルされた時に仮売上をキャンセル失敗した時
	STATUS_FAILED_CONSENT               = -2 //オーダーが同意されなかった時
	STATUS_FAILED_PROVISION_SALE        = -1 //仮売上が取れなかった時
	STATUS_GET_PROVISION_SALE           = 1  //仮売上をとった
	STATUS_GET_CONSENT                  = 2  //同意が取れた時
	STATUS_CANCEL                       = 3  //キャンセルされた時
	STATUS_GET_REAL_SALE                = 4  //実売上をとった
	STATUS_WRITE_ON_CSV                 = 5  //CSVに書き出し
	STATUS_CONTINUE_DELAY               = 6  //遅延キャンセルをキャンセルして続ける場合
	STATUS_CONTINUE_DELAY_GET_REAL_SALE = 7  //遅延キャンセルをして実売上をとった場合
	STATUS_FINISH                       = 99 //すべての工程を終了
)

//利用日の次の日が44日目(仮売上期限の一日前)->利用日は仮売上期限の二日前
const DAY_LIMIT int = 43

//キャンセルの日
const CANCEL_FREE_DAY_LIMIT int = 4

//キャンセル料の割合
const CANCEL_RATE float64 = 0.2

type orderType struct {
	Order_id           int         `db:order_id`
	Order_charge_id    string      `db:order_charge_id`
	Order_consent      int         `db:order_consent`
	Transport_allocate int         `db:transport_allocate`
	Rental_from        interface{} `db:rental_from`
	Rental_to          interface{} `db:rental_to`
	Item_id            int         `db:item_id`
	User_id            int         `db:user_id`
	Day_price          int         `db:day_price`
	Amount             int         `db:amount`
	Cancel_date        interface{} `db:cancel_date`
	Cancel_status      int         `db:cancel_status`
	Status             int         `db:status`
}

func getOrderInfo(orderID string, db *sql.DB) (*orderType, error) {
	dbSql := fmt.Sprintf("SELECT * FROM %v WHERE %v=?", ORDER, ORDER_ID)
	//stmt, err := db.Prepare(dbSql)
	//res, err := stmt.Exec(orderID)
	res, err := db.Query(dbSql, orderID)
	order := new(orderType)
	if err != nil {
		return order, err
	}
	for res.Next() {
		if err := res.Scan(&order.Order_id,
			&order.Order_charge_id,
			&order.Order_consent,
			&order.Transport_allocate,
			&order.Rental_from,
			&order.Rental_to,
			&order.Item_id,
			&order.User_id,
			&order.Day_price,
			&order.Amount,
			&order.Cancel_date,
			&order.Cancel_status,
			&order.Status); err != nil {
			return order, err
		}
	}
	return order, nil
}

func checkCanDelayCancelDay(orderID string, db *sql.DB) (bool, error) {
	dbSql := fmt.Sprintf("SELECT %v FROM %v where %v=%v", RENTAL_FROM, ORDER, ORDER_ID, orderID)
	var rentalFrom interface{}
	res, err := db.Query(dbSql)
	if err != nil {
		return false, err
	}
	for res.Next() {
		if err := res.Scan(&rentalFrom); err != nil {
			return false, err
		}
	}

	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), nowTime.Minute(), nowTime.Second(), time.UTC)
	fmt.Printf("day: %v\n now: %v\n", rentalFrom, nowTime)
	subTime := nowTime.Sub(rentalFrom.(time.Time))
	fmt.Printf("sub: %v \n", subTime)
	if subTime.Hours() >= 24 || subTime.Hours() < 0 {
		return false, nil
	}
	return true, nil
}

func getItemID(orderID string, db *sql.DB) (string, error) {
	dbSql := fmt.Sprintf("SELECT %v FROM %v where %v=%v", ITEM_ID, ORDER, ORDER_ID, orderID)
	var itemID int
	res, err := db.Query(dbSql)
	if err != nil {
		return "", err
	}
	for res.Next() {
		if err := res.Scan(&itemID); err != nil {
			return "", err
		}
	}
	return strconv.Itoa(itemID), nil
}

/**
 * おーだーIDからchargeIDを取得
 * @param  {[type]} orderID int           [description]
 * @param  {[type]} db      *sql.DB)      (string,      error [description]
 * @return {[type]}         [description]
 */
func getChargeID(orderID string, db *sql.DB) (string, error) {
	dbSql := fmt.Sprintf("SELECT order_charge_id FROM orders where order_id=%v", orderID)
	var chargeID string
	res, err := db.Query(dbSql)
	if err != nil {
		return "", err
	}
	for res.Next() {
		if err := res.Scan(&chargeID); err != nil {
			return "", err
		}
	}
	return chargeID, nil
}

/**
 * orderIDから料金を取得
 * @param  {[type]} orderID string        [description]
 * @param  {[type]} db      *sql.DB)      (int,         error [description]
 * @return {[type]}         [description]
 */
func getAmount(orderID string, db *sql.DB) (int, error) {
	dbSql := fmt.Sprintf("SELECT %v FROM %v where %v=%v", ORDER_AMOUNT, ORDER, ORDER_ID, orderID)
	var amount int
	res, err := db.Query(dbSql)
	if err != nil {
		return 0, err
	}
	for res.Next() {
		if err := res.Scan(&amount); err != nil {
			return 0, err
		}
	}
	return amount, nil
}

func getOrderStatus(orderID string, db *sql.DB) (int, error) {
	var state int
	dbSql := fmt.Sprintf("SELECT %v FROM %v WHERE %v=%v", ORDER_STATUS, ORDER, ORDER_ID, orderID)
	res, err := db.Query(dbSql)
	if err != nil {
		return 0, err
	}
	for res.Next() {
		if err := res.Scan(&state); err != nil {
			return 0, err
		}
	}
	return state, nil
}

//オーダーのステータスの変更
func updateOrderState(orderID string, state int, db *sql.DB) error {
	dbSql := fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_STATUS, ORDER_ID)
	stmt, err := db.Prepare(dbSql)
	_, err = stmt.Exec(state, orderID)
	return err
}

//キャンセルオーダーの設定
func updateCancelOrderState(orderID string, state int, db *sql.DB) error {
	dbSql := fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_CANCEL_STATE, ORDER_ID)
	stmt, err := db.Prepare(dbSql)
	_, err = stmt.Exec(state, orderID)
	return err
}
