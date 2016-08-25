package account

import (
	"database/sql"
	"fmt"
	"strconv"
)


func checkUserID(userIDStr string) bool {
	_, err := strconv.Atoi(userIDStr)
	if err != nil {
		return false
	}
	return true
}
/**
 * ユーザーIDからカスタマーIDを出す
 * @param  {[type]} userID int           [description]
 * @param  {[type]} db     *sql.DB)      (string,      error [description]
 * @return {[type]}        [description]
 */
func getCustomerID(userID int, db *sql.DB) (string, error) {
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
