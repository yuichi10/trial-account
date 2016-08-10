package account

import (
	"database/sql"
	"fmt"
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
	ORDER_STATE_CANCEL_NONE  = "0" //キャンセル無し
	ORDER_STATE_CANCEL_FREE  = "1" //無料のキャンセル
	ORDER_STATE_CANCEL_PAID  = "2" //有料のキャンセル
	ORDER_STATE_CANCEL_DELAY = "3"
)

type orderType struct {
	Order_id           int         `db:order_id`
	Order_charge_id    string      `db:order_charge_id`
	order_consent      int         `db:order_consent`
	Transport_allocate int         `db:transport_allocate`
	Rental_from        interface{} `db:rental_from`
	Rental_to          interface{} `db:rental_to`
	Item_id            int         `db:item_id`
	User_id            int         `db:user_id`
	Day_price          int         `db:day_price`
	Amount             int         `db:amount`
	Deposit_id         int         `db:deposit_id`
	Cancel_date        interface{} `db:cancel_date`
	Cancel_status      int         `db:cancel_status`
	Status             int         `db:status`
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
	fmt.Printf("day: %v\nnow: %v\n", rentalFrom, nowTime)
	subTime := nowTime.Sub(rentalFrom.(time.Time))
	fmt.Printf("sub: %v \n", subTime)
	if subTime.Hours() >= 24 || subTime.Hours() < 0 {
		return false, nil
	}
	return true, nil
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
