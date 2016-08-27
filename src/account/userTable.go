package account

import (
	"database/sql"
	"fmt"
)

const (
	USER = "users"
	USER_ID = "user_id"
	USER_CREDIT_ID = "credit_customer_id"
	USER_NAME = "name"
)

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
