package account

import (
	"database/sql"
	"fmt"
)

const (
	DEPOSIT_STATE_UPDATE = 1
	DEPOSIT_STATE_FINISH = 99
)

const (
	DEPOSIT               = "deposits"
	DEPOSIT_CHARGE_ID     = "deposit_charge_id"
	DEPOSIT_DEPOSIT_PRICE = "deposit_price"
	DEPOSIT_DELAY_PRICE   = "delay_price"
	DEPOSIT_DELAY_DAY     = "delay_day"
	DEPOSIT_AMOUNT        = "amount"
	DEPOSIT_STATUS        = "status"
)

type depositType struct {
	Order_id          int         `db:order_id`
	Deposit_charge_id interface{} `db:deposit_charge_id`
	Deposit_price     int         `db:deposit_price`
	Delay_price       int         `db:delay_price`
	Delay_day         float64     `db:delay_day`
	Amount            int         `db:amount`
	Status            int         `db:status`
}

//func GetDepositInfo(w http.ResponseWriter, r *http.Request) {
//
//}
func getDepositInfo(orderID string, db *sql.DB) (*depositType, error) {
	dbSql := fmt.Sprintf("SELECT * FROM %v WHERE %v=%v", DEPOSIT, ORDER_ID, orderID)
	res, err := db.Query(dbSql)
	if err != nil {
		return new(depositType), err
	}
	deposit := new(depositType)
	for res.Next() {
		if err := res.Scan(&deposit.Order_id, &deposit.Deposit_charge_id, &deposit.Deposit_price, &deposit.Delay_price, &deposit.Delay_day, &deposit.Amount, &deposit.Status); err != nil {
			return new(depositType), err
		}
	}
	return deposit, nil
}

func calcDepositAmount(deposit *depositType) int {
	return (int)((float64)(deposit.Deposit_price) + (float64)(deposit.Delay_price)*deposit.Delay_day)
}
