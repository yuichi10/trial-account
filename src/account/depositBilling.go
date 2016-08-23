package account

import (
	_ "database/sql"
	"dbase"
	_ "encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/http"
	"strconv"
	_ "time"
)

/**
 * 保険料のデポジットの作成
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func StartNegotiateDeposit(w http.ResponseWriter, r *http.Request) {
	//理由から遅延料金、保険料金を設定
	//保険料のデポジットをとれる期間かどうかの判定,もうすでにないかどうかの判定
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	orderID := r.Form.Get(ORDER_ID)
	iOrderId, _ := strconv.Atoi(orderID)
	itemID, _ := getItemID(orderID, db)
	item, _ := getItemData(itemID, db)
	if !checkOrderStatus(orderID, db, []int{STATUS_GET_REAL_SALE, STATUS_GET_CONSENT}...) {
		//オーダーをチェックする
		fmt.Fprintf(w, "ステータスの影響で作れません")
		return
	}
	if !checkDepositLimit(orderID, db) {
		fmt.Fprintf(w, "その期間はデポジットを作れません")
		return
	}
	fmt.Fprintf(w, "Itme: %v \n", item)
	deposit := new(depositType)
	deposit.Order_id = iOrderId
	deposit.Deposit_price = item.Deposit_price
	deposit.Delay_price = item.Delay_price
	deposit.Delay_day = 1
	deposit.Status = DEPOSIT_STATE_UPDATE
	deposit.Amount = (int)((float64)(deposit.Deposit_price) + (float64)(deposit.Delay_price)*deposit.Delay_day)
	dbSql := fmt.Sprintf("INSERT %v SET %v=?, %v=?, %v=?, %v=?, %v=?, %v=?", DEPOSIT, ORDER_ID, DEPOSIT_DEPOSIT_PRICE, DEPOSIT_DELAY_PRICE, DEPOSIT_DELAY_DAY, DEPOSIT_AMOUNT, DEPOSIT_STATUS)
	stmt, err := db.Prepare(dbSql)
	if err != nil {
		fmt.Fprintf(w, "プリペアエラー: %v", err)
		return
	}
	_, err = stmt.Exec(iOrderId, deposit.Deposit_price, deposit.Delay_price, deposit.Delay_day, deposit.Amount, deposit.Status)
	if err != nil {
		fmt.Fprintf(w, "exec err: %v", err)
		return
	}
	fmt.Fprintf(w, "新しいデポジットを作成しました")
}

/**
 * デポジット料金をアップデート
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func UpdateDeposit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	//両方共同意していない時だけ、していた場合はアップデート出来ない
	orderID := r.Form.Get(ORDER_ID)
	deposit, err := getDepositInfo(orderID, db)
	if depositPrice := r.Form.Get(DEPOSIT_DEPOSIT_PRICE); depositPrice != "" {
		deposit.Deposit_price, _ = strconv.Atoi(depositPrice)
	}
	if depositDelayPrice := r.Form.Get(DEPOSIT_DELAY_PRICE); depositDelayPrice != "" {
		deposit.Delay_price, _ = strconv.Atoi(depositDelayPrice)
	}
	if depositDelayDay := r.Form.Get(DEPOSIT_DELAY_DAY); depositDelayDay != "" {
		deposit.Delay_day, _ = strconv.ParseFloat(depositDelayDay, 64)
	}
	deposit.Amount = calcDepositAmount(deposit)
	dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=?, %v=?, %v=?, %v=? WHERE %v=?", DEPOSIT, DEPOSIT_DEPOSIT_PRICE, DEPOSIT_DELAY_PRICE, DEPOSIT_DELAY_DAY, AMOUNT, DEPOSIT_STATUS, ORDER_ID)
	stmt, _ := db.Prepare(dbSql)
	_, err = stmt.Exec(deposit.Deposit_price, deposit.Delay_price, deposit.Delay_day, deposit.Amount, DEPOSIT_STATE_UPDATE, orderID)
	if err != nil {
		fmt.Fprintf(w, "アップデートエラー: %v \n", err)
		return
	}
	deposit, err = getDepositInfo(orderID, db)
	fmt.Fprintf(w, "デポジット: %v \n エラー: %v \n", deposit, err)
	return
}

/**
 * デポジット料金の同意(両方)
 * @param  {[type]} w http.ResponseWriter [description]
 * @param  {[type]} r *http.Request       [description]
 * @return {[type]}   [description]
 */
func ConsentDeposit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	orderID := r.Form.Get(ORDER_ID)
	itemID := r.Form.Get(ITEM_ID)
	userID := r.Form.Get(USER_ID)
	order, err := getOrderInfo(orderID, db)
	deposit, err := getDepositInfo(orderID, db)
	var dbSql string
	if userID != "" && userID == strconv.Itoa(order.User_id) {
		fmt.Fprintln(w, "ユーザー")
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", DEPOSIT, DEPOSIT_STATUS, ORDER_ID)
		stmt, err := db.Prepare(dbSql)
		if err != nil {
			fmt.Fprintf(w, "プリペアエラ-: %v \n", err)
			return
		}
		if deposit.Status == DEPOSIT_STATE_UPDATE {
			fmt.Fprintln(w, "片方")
			_, err = stmt.Exec(DEPOSIT_STATE_RENT_AGREE, orderID)
			if err != nil {
				fmt.Fprintf(w, "Exec: %v \n", err)
				return
			}
		} else if deposit.Status == DEPOSIT_STATE_LEND_AGREE {
			fmt.Fprintln(w, "両方")
			_, err = stmt.Exec(DEPOSIT_STATE_BOTH_AGREE, orderID)
			if err != nil {
				fmt.Fprintf(w, "Exec: %v \n", err)
				return
			}
		}
	}
	if itemID != "" && itemID == strconv.Itoa(order.Item_id) {
		fmt.Fprintln(w, "かす側")
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", DEPOSIT, DEPOSIT_STATUS, ORDER_ID)
		stmt, err := db.Prepare(dbSql)
		if err != nil {
			fmt.Fprintf(w, "プリペアエラ-: %v \n", err)
			return
		}
		if deposit.Status == DEPOSIT_STATE_UPDATE {
			fmt.Fprintln(w, "片方")
			_, err = stmt.Exec(DEPOSIT_STATE_LEND_AGREE, orderID)
			if err != nil {
				fmt.Fprintf(w, "Exec: %v \n", err)
				return
			}
		} else if deposit.Status == DEPOSIT_STATE_RENT_AGREE {
			fmt.Fprintln(w, "両方")
			_, err = stmt.Exec(DEPOSIT_STATE_BOTH_AGREE, orderID)
			if err != nil {
				fmt.Fprintf(w, "Exec: %v \n", err)
				return
			}
		}
	}
	// 両方アグリーの時仮売上を取る
	deposit, err = getDepositInfo(orderID, db)
	if deposit.Status == DEPOSIT_STATE_BOTH_AGREE {
		cusID, err := getCustomerID(order.User_id, db)
		rawjson, err := webpayCreateProvisionalSale(cusID, strconv.Itoa(deposit.Amount), r)
		if err != nil {
			updateDepositState(orderID, DEPOSIT_STATE_FAILED_PROVISION_SALE, db)
			return
		}
		js, _ := simplejson.NewJson([]byte(rawjson))
		if val := js.Get(WP_ERROR).Interface(); val != nil {
			fmt.Fprintf(w, "トークンにエラーがありました: %v\n メッセージ%v\n", val, js.Get(WP_ERROR).Get("message").MustString())
			updateDepositState(orderID, DEPOSIT_STATE_FAILED_PROVISION_SALE, db)
			return
		} else {
			dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", DEPOSIT, DEPOSIT_CHARGE_ID, ORDER_ID)
			stmt, err := db.Prepare(dbSql)
			_, err = stmt.Exec(js.Get(WP_ID).MustString(), orderID)
			if err != nil {
				fmt.Fprintf(w, "仮売上チャージIDの情報をセットエラー: %v ", err)
				return
			}
			//仮売上の取得
			updateDepositState(orderID, DEPOSIT_STATE_GET_PROVISON_SALE, db)
			//実売上を取る
			rawjson, err := webpayProvisionalToReal(js.Get(WP_ID).MustString(), strconv.Itoa(deposit.Amount), r)
			js, _ := simplejson.NewJson([]byte(rawjson))
			if isErr, wpErr := checkCardError(js); !isErr {
				fmt.Fprintf(w, "実売上を取れませんでした: %v \n", wpErr)
				return
			} else {
				updateDepositState(orderID, DEPOSIT_STATE_GET_REAL_SALE, db)
				fmt.Fprintln(w, "デポジットを終了しました")
			}

		}
	}
	fmt.Fprintf(w, "オーダー: %v \n エラー: %v \n", order, err)
}
