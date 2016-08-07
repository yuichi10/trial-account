package account

import (
	"database/sql"
	"fmt"
)

const (
	//オーダー
	ORDER = "orders"
	//オーダーID
	ORDER_ID = "order_id"
	//レンタルの開始日
	RENTAL_FROM = "rental_from"
	//レンタル終了日
	RENTAL_TO       = "rental_to"
	ORDER_CHARGE_ID = "order_charge_id"
	ORDER_CONSENT   = "order_consent"
	ORDER_STATUS    = "status"
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
	Is_cancel          int         `db:is_cancel`
	Status             int         `db:status`
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
