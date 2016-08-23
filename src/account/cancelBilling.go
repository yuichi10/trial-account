package account

import (
	"database/sql"
	"dbase"
	_ "encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/http"
	"strconv"
	"time"
)

/**
 * オーダーのキャンセル(借りる側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func CanselOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	orderID := r.Form.Get(ORDER_ID)
	if checkOrderStatus(orderID, db, []int{STATUS_GET_PROVISION_SALE}...) {
		//まだ相手が同意してない時は無料でキャンセル
		fmt.Fprintf(w, "まだ同意していないため無料でのキャンセル")
		freeCancel(orderID, db, r)
		return
	}

	if !checkOrderStatus(orderID, db, []int{STATUS_GET_CONSENT}...) {
		fmt.Fprintf(w, "ステータスによりキャンセルできません\n")
		return
	}
	res, err := canCancelFree(orderID, db)
	if err != nil {
		fmt.Fprintf(w, "借りれるかエラー%v \n", err)
		return
	}
	fmt.Fprintf(w, "レンタル日: %v \n ERR: %v \n", res, err)
	//キャンセルをtrueに
	if res {
		//キャンセル料がかからない時
		freeCancel(orderID, db, r)
	} else {
		//キャンセル料がかかるとき
		payCancel(orderID, db, r)
	}
}

/**
 * 借りる側が商品が遅れたことによるキャンセルまたは続行のレポート(利用日初日の24時間のみ)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func DelayCanselReport(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	orderID := r.Form.Get(ORDER_ID)
	isCancelStr := r.Form.Get(IS_CANCEL)
	isCancel, _ := strconv.Atoi(isCancelStr)
	if !checkOrderStatus(orderID, db, []int{STATUS_GET_CONSENT}...) {
		fmt.Fprintf(w, "ステータスの影響でできません \n")
		return
	}
	is, err := checkCanDelayCancelDay(orderID, db)
	if !is || err != nil {
		fmt.Fprintf(w, "できません.\nERR: %v\n", err)
		return
	}
	fmt.Fprintf(w, "キャンセル: %v\n", isCancel)
	if isCancel != 0 {
		//キャンセルを選んだ時
		delayChooseCancel(orderID, db, r)
	} else {
		//使い続ける時
		delayChooseContinue(orderID, db, r)
	}
}

//無料のキャンセル
func freeCancel(orderID string, db *sql.DB, r *http.Request) {
	t := time.Now()
	dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_STATUS, ORDER_ID)
	stmt, err := db.Prepare(dbSql)
	if err != nil {
		fmt.Printf("str: %v \nstmt ERR: %v \n", dbSql, err)
		return
	}
	_, err = stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_FREE, STATUS_CANCEL, orderID)
	if err != nil {
		fmt.Printf("実行のエラー: %v \n ", err)
		return
	}
	chID, err := getChargeID(orderID, db)
	if err != nil {
		fmt.Printf("chargeID: %v \n ", err)
		return
	}
	rawjson, err := webpayCancelProvisionalSale(chID, r)
	if err != nil {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_CANCEL_FREE, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
			return
		}
	}
	js, _ := simplejson.NewJson([]byte(rawjson))
	if ok, _ := checkCardError(js); !ok {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_CANCEL_FREE, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
		}
		return
	}
	if err := updateCancelOrderState(orderID, ORDER_STATE_CANCEL_FREE, db); err != nil {
		fmt.Printf("キャンセルアップデートエラー: %v \n ", err)
		return
	}
}

//有料のキャンセル
func payCancel(orderID string, db *sql.DB, r *http.Request) {
	t := time.Now()
	dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_STATUS, ORDER_ID)
	stmt, _ := db.Prepare(dbSql)
	_, err := stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_PAID, STATUS_CANCEL, orderID)
	if err != nil {
		fmt.Printf("実行のエラー: %v \n ", err)
		return
	}
	chID, err := getChargeID(orderID, db)
	amount, err := getAmount(orderID, db)
	var amountFloat float64 = CANCEL_RATE * float64(amount)
	amount = int(amountFloat)
	rawjson, err := webpayProvisionalToReal(chID, strconv.Itoa(amount), r)
	if err != nil {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_CANCEL_PAY, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
			return
		}
	}
	js, _ := simplejson.NewJson([]byte(rawjson))
	if ok, _ := checkCardError(js); !ok {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_CANCEL_PAY, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
		}
		return
	}
	if err := updateCancelOrderState(orderID, ORDER_STATE_CANCEL_PAID, db); err != nil {
		fmt.Printf("キャンセルアップデートエラー: %v \n ", err)
		return
	}
}

//キャンセルを無料でできるかどうか
func canCancelFree(orderID string, db *sql.DB) (bool, error) {
	dbSql := fmt.Sprintf("SELECT %v FROM %v WHERE %v=?", RENTAL_FROM, ORDER, ORDER_ID)
	res, err := db.Query(dbSql, orderID)
	if err != nil {
		return false, err
	}
	var rentalStartDateInter interface{}
	for res.Next() {
		if err := res.Scan(&rentalStartDateInter); err != nil {
			return false, err
		}
	}
	if rentalStartDateInter == nil {
		return false, nil
	}
	//rentalStartDate := time.Time(rentalStartDateStr)
	rentalStartDate := rentalStartDateInter.(time.Time)
	subDays := calcSubDate(time.Now(), rentalStartDate)
	fmt.Printf("かすまでの日数: %v \n ", subDays)
	if subDays <= CANCEL_FREE_DAY_LIMIT {
		return false, nil
	}
	return true, nil
}

//遅れによってキャンセルを選んだ時
func delayChooseCancel(orderID string, db *sql.DB, r *http.Request) {
	t := time.Now()
	dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_STATUS, ORDER_ID)
	stmt, _ := db.Prepare(dbSql)
	_, err := stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_DELAY, STATUS_CANCEL, orderID)
	if err != nil {
		fmt.Printf("実行のエラー: %v \n ", err)
		return
	}
	chID, err := getChargeID(orderID, db)
	if err != nil {
		fmt.Printf("chargeID: %v \n ", err)
		return
	}
	rawjson, err := webpayCancelProvisionalSale(chID, r)
	if err != nil {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_DELAY_CANCEL, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
			return
		}
	}
	js, _ := simplejson.NewJson([]byte(rawjson))
	if ok, _ := checkCardError(js); !ok {
		if err := updateCancelOrderState(orderID, STATUS_FAILED_DELAY_CANCEL, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
		}
		return
	}
	if err := updateCancelOrderState(orderID, ORDER_STATE_CANCEL_DELAY, db); err != nil {
		fmt.Printf("キャンセルアップデートエラー: %v \n ", err)
		return
	}
	return
}

//遅れても使うとき
func delayChooseContinue(orderID string, db *sql.DB, r *http.Request) {
	if err := updateOrderState(orderID, STATUS_CONTINUE_DELAY, db); err != nil {
		fmt.Printf("アップデータのエラー: %v \n", err)
		return
	}
	chID, err := getChargeID(orderID, db)
	amount, err := getAmount(orderID, db)
	if err != nil {
		fmt.Printf("chargeID: %v \n ", err)
		return
	}
	rawjson, err := webpayProvisionalToReal(chID, strconv.Itoa(amount), r)
	if err != nil {
		if err := updateOrderState(orderID, STATUS_CONTINUE_DELAY_FAILED, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
			return
		}
	}
	js, _ := simplejson.NewJson([]byte(rawjson))
	if ok, _ := checkCardError(js); !ok {
		if err := updateOrderState(orderID, STATUS_CONTINUE_DELAY_FAILED, db); err != nil {
			fmt.Printf("webpayエラー: %v \n ", err)
		}
		return
	}
	if err := updateOrderState(orderID, STATUS_CONTINUE_DELAY, db); err != nil {
		fmt.Printf("キャンセルアップデートエラー: %v \n ", err)
		return
	}
	return
}
