package account

import (
	"database/sql"
	_ "encoding/json"
	"fmt"
	"time"
	//"github.com/bitly/go-simplejson"
)

const (
	DEPOSIT_STATE_FAILED_PROVISION_SALE = -5
	DEPOSIT_STATE_FAILED_REAL_SALE      = -6
	DEPOSIT_STATE_FAILED_WRITE_CSV      = -7
	DEPOSIT_STATE_UPDATE                = 1 //アップデート中
	DEPOSIT_STATE_RENT_AGREE            = 2
	DEPOSIT_STATE_LEND_AGREE            = 3
	DEPOSIT_STATE_BOTH_AGREE            = 4
	DEPOSIT_STATE_GET_PROVISON_SALE     = 5
	DEPOSIT_STATE_GET_REAL_SALE         = 6
	DEPOSIT_STATE_WRITE_CSV             = 7
	DEPOSIT_STATE_FINISH                = 99
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

//いまのところリミットは４日
const DEPOSIT_TIME_PERIOD = 5759

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

func updateDepositState(orderID string, state int, db *sql.DB) error {
	dbSql := fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", DEPOSIT, DEPOSIT_STATUS, ORDER_ID)
	stmt, err := db.Prepare(dbSql)
	_, err = stmt.Exec(state, orderID)
	return err
}

func calcDepositAmount(deposit *depositType) int {
	return (int)((float64)(deposit.Deposit_price) + (float64)(deposit.Delay_price)*deposit.Delay_day)
}

func checkDepositLimit(orderID string, db *sql.DB) bool {
	dbSql := fmt.Sprintf("SELECT %v FROM %v WHERE %v=%v", RENTAL_TO, ORDER, ORDER_ID, orderID)
	res, err := db.Query(dbSql)
	if err != nil {
		fmt.Printf("checkDepositLimitERR: %v \n", err)
		return false
	}
	var to interface{}
	for res.Next() {
		if err := res.Scan(&to); err != nil {
			fmt.Printf("read SQL DEPOSIT LIMIT ERR: %v \n", err)
			return false
		}
	}
	limitDay := to.(time.Time).AddDate(0, 0, 5)
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), nowTime.Second(), nowTime.Nanosecond(), time.UTC)
	subTime := limitDay.Sub((nowTime))
	fmt.Printf("nowTime: %v \n limitDay: %v \n to: %v \n ", nowTime, limitDay, subTime.Minutes())
	if subTime.Minutes() > DEPOSIT_TIME_PERIOD || subTime.Minutes() < 0 {
		return false
	}
	return true
}
