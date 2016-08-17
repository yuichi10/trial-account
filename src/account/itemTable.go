package account

import (
	"database/sql"
	"fmt"
)

const (
	ITEM    = "items"
	ITEM_ID = "item_id"
)

//アイテムデータ
type itemData struct {
	Item_id       int    `db:item_id`
	User_id       int    `db:user_id`
	Product_name  string `db:product_name`
	Oneday_price  int    `db:oneday_price`
	Longday_price int    `db:longday_price`
	Deposit_price int    `db:deposit_price`
	Delay_price   int    `db:delay_price`
}

func getItemData(itemID string, db *sql.DB) (*itemData, error) {
	dbSql := fmt.Sprintf("SELECT * FROM %v WHERE %v=%v", ITEM, ITEM_ID, itemID)
	item := new(itemData)
	res, err := db.Query(dbSql)
	if err != nil {
		return item, err
	}
	for res.Next() {
		if err := res.Scan(&item.Item_id, &item.User_id, &item.Product_name, &item.Oneday_price, &item.Longday_price, &item.Deposit_price, &item.Delay_price); err != nil {
			return item, err
		}
	}
	return item, nil
}
