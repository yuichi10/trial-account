package account

import (
	"database/sql"
	"dbase"
	_ "encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/http"
	_ "reflect"
	"strconv"
	"strings"
	"time"
)

//insert into orders (transport_allocate,rental_from,rental_to, item_id, user_id, day_price)
//insert into items(user_id, product_name, oneday_price, longday_price, deposit_price, delay_price) values (3, 'Nikon', 5000, 4000, 10000, 6000)

///orderに料金を設定する理由は後でプロダクトの値段が変わったらレシートの意味がないから
//orderのお金が仮売上→実売上のステータスを持っておくべき
//depositもお金をとったかどうかのステータスを持っておくべき

//キャンセル0 キャンセルなし
//キャンセル1　通常キャンセル
//キャンセル2 遅延によるキャンセル

//ステータス
//オーダーの同意がキャンセルもしくは仮売上が取得できなかった時-1
//オーダーが存在している0
//仮売上をとった1
//実売上をとった2
//デポジットをを考えている 3
//すべての工程が終了

const (
	//トークン
	TOKEN = "token"
	//ユーザーID
	USER_ID = "user_id"
)

const (
	//オーダーステータス
	STATUS_FAILED                 = "-1" //オーダーが同意されなかった時
	STATUS_GET_PROVISION_SALE     = "1"  //仮売上をとった
	STATUS_GET_REAL_SALE          = "2"  //実売上をとった
	STATUS_DELAY_CANCEL_REAL_SALE = "3"  //遅延によるキャンセルで実売上をとった
	STATUS_FINISH                 = "99" //すべての工程を終了
)

//利用日の次の日が44日目(仮売上期限の一日前)->利用日は仮売上期限の二日前
const DAY_LIMIT int = 43

//キャンセルの日
const CANCEL_FREE_DAY_LIMIT int = 4

//キャンセル料の割合
const CANCEL_RATE float64 = 0.2

func TestDB(w http.ResponseWriter, r *http.Request) {

	db, err := dbase.OpenDbr()
	if err != nil {
		fmt.Fprintf(w, "オープンERR: %v \n", err)
	}
	tx, err := db.Begin()
	if err != nil {
		fmt.Fprintf(w, "トランザクションエラー: %v \n", err)
		return
	}
	_, err = db.InsertInto("items").
		Columns("user_id", "product_name", "oneday_price", "longday_price", "deposit_price", "delay_price").
		Values(3, "Canon E 10", 4800, 4200, 12000, 7000).
		Exec()
	if err != nil {
		tx.Rollback()
		return
	} else {

		err := tx.Rollback()
		fmt.Fprintf(w, "できたけどロールバック: %v \n", err)
		return
	}
	var ot []orderType

	db.Select("*").From("items").Load(&ot)
	fmt.Fprintf(w, "製品一覧 %v", ot[0])
	tx.Commit()
	//db := dbase.OpenDB()
	//defer db.Close()
	//tx, err := db.Begin()
	//if err != nil {
	//	fmt.Fprintf(w, "トランザクションエラー")
	//	return
	//}
	//dbSql := "INSERT INTO items (user_id, product_name, oneday_price, longday_price, deposit_price, delay_price) VALUES(?, ?, ?, ?, ?, ?)"
}

/**
 * カスタマーの追加
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func AddCustomer(w http.ResponseWriter, r *http.Request) {
	db := dbase.OpenDB()
	defer db.Close()
	r.ParseForm()

	token := r.Form.Get(WP_TOKEN)
	rawjson, err := WebpayAddclient(token, r)
	if err != nil {
		return
	}
	js, _ := simplejson.NewJson([]byte(rawjson))

	if val := js.Get(WP_ERROR).Interface(); val != nil {
		fmt.Fprintf(w, "トークンにエラーがありました: %v\n メッセージ%v\n", val, js.Get(WP_ERROR).Get("message").MustString())
		return
	}
	cusID := js.Get(WP_CUS_ID).MustString()
	cusName := js.Get(WP_CUS_CARD).Get(WP_CUS_CARD_NAME).MustString()
	last4 := js.Get(WP_CUS_CARD).Get(WP_CUS_CURD_LAST4).MustString()
	//ユーザーの追加
	dbSql := fmt.Sprintf("INSERT users SET %s=?, %s=?", dbase.USER_CUSTMER_ID, dbase.USER_NAME)
	stmt, _ := db.Prepare(dbSql)
	_, err = stmt.Exec(cusID, cusName)
	if err == nil {
		fmt.Fprintf(w, "新しいユーザー %v が追加されました.\n　クレジットカードは**** **** %v で登録しました。\nCustomer ID は %v です\n", cusName, last4, cusID)
	} else {
		fmt.Fprintf(w, "ユーザー作成に失敗しました ERROR: %v", err)
	}
}

/**
 * オーダーを作成する
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func PublishOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	//借りる側のユーザーID
	userID := r.Form.Get(USER_ID)
	//借りるアイテム
	itemID := r.Form.Get(ITEM_ID)
	//レンタル期間
	rTo := r.Form.Get(RENTAL_TO)
	rFrom := r.Form.Get(RENTAL_FROM)
	//現在時刻
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//レンタル開始日
	y, m, d := divideTime(rFrom)
	rentalFrom := time.Date(y, time.Month(m), d, nowTime.Hour(), nowTime.Minute(), nowTime.Second(), 0, time.UTC)
	//レンタル終了日
	var rentalTo time.Time
	if rTo == "" {
		//最後の日が指定してなかった場合
		rentalTo = rentalFrom
		rTo = rFrom
	} else {
		y, m, d = divideTime(rTo)
		rentalTo = time.Date(y, time.Month(m), d, nowTime.Hour(), nowTime.Minute(), nowTime.Second(), 0, time.UTC)
	}
	//利用できる日かどうか
	if !checkRentalDay(rentalFrom, rentalTo, itemID, db) {
		fmt.Fprintf(w, "%vから%vはレンタルできません\n", rentalFrom, rentalTo)
		return
	}

	//アイテムの情報を取得(料金などを設定するため)
	iData := new(itemData)
	dbSql := fmt.Sprintf("SELECT * FROM items where %v=%v", ITEM_ID, itemID)
	res, err := db.Query(dbSql)
	if err != nil {
		fmt.Fprintf(w, "%v \n select item ERR: %v\n", dbSql, err)
		return
	}
	for res.Next() {
		if err := res.Scan(&iData.Item_id, &iData.User_id, &iData.Product_name, &iData.Oneday_price, &iData.Longday_price, &iData.Deposit_price, &iData.Delay_price); err != nil {
			fmt.Fprintf(w, "scan item err: %v", err)
			return
		}
	}
	fmt.Fprintf(w, "プロダクトデータ: %v \n", iData)

	//料金を設定
	var dayPrice int
	var amount int
	if period := calcSubDate(rentalFrom, rentalTo); (period + 1) > 1 {
		dayPrice = iData.Longday_price
		amount = (period + 1) * dayPrice
	} else {
		dayPrice = iData.Oneday_price
		amount = dayPrice
	}
	fmt.Fprintf(w, "一日の料金は%v 合計料金は%v　です。\n", dayPrice, amount)
	//新しいオーダーの作成
	dbSql = "INSERT orders SET transport_allocate=?, rental_from=?, rental_to=?, item_id=?, user_id=?, day_price=?, amount=?"
	stmt, _ := db.Prepare(dbSql)
	insRes, err := stmt.Exec(0, rFrom, rTo, itemID, userID, dayPrice, amount)
	if err != nil {
		fmt.Fprintf(w, "オーダーのエラー: %v \n", err)
		return
	}
	orderLastID, _ := insRes.LastInsertId()
	fmt.Fprintf(w, "オーダを作りました\n プロダクトIDは%v\n オーダーのIDは%v\n", itemID, orderLastID)
	//仮売上を取得
	userID_int, _ := strconv.Atoi(userID)
	cID, _ := getCustomerID(userID_int, db)
	if err != nil {
		fmt.Fprintf(w, "customer id err: %v \n", err)
		return
	}
	fmt.Fprintf(w, "customer ID: %v\n", cID)
	wpcRawJson, _ := webpayCreateProvisionalSale(cID, strconv.Itoa(amount), r)
	jsJson, _ := simplejson.NewJson([]byte(wpcRawJson))
	//ウェブペイのIDを登録
	dbSql = "UPDATE orders SET order_charge_id=?, status=? where order_id=?"
	stmt, _ = db.Prepare(dbSql)
	_, err = stmt.Exec(jsJson.Get(WP_ID).MustString(), STATUS_GET_PROVISION_SALE, orderLastID)
	if err != nil {
		fmt.Fprintf(w, "アップデート: %v \n", err)
		return
	}
}

/**
 * オーダーに同意(かす側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func ConsentOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	//オーダーのID
	orderID := r.Form.Get(ORDER_ID)
	//Orderのconsentをtrueに
	dbSql := fmt.Sprintf("UPDATE %v SET %v=? where %v=?", ORDER, ORDER_CONSENT, ORDER_ID)
	stmt, _ := db.Prepare(dbSql)
	_, err := stmt.Exec(true, orderID)
	if err != nil {
		fmt.Fprintf(w, "UPDATE ERROR: %v\n", err)
		return
	}
	fmt.Fprintf(w, "同意されました。\n")
}

/**
 * オーダーが同意しなかった時
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func DisagreeOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	defer db.Close()
	orderID := r.Form.Get(ORDER_ID)
	dbSql := fmt.Sprintf("UPDATE %v SET %v=? where %v=?", ORDER, ORDER_STATUS, ORDER_ID)
	stmt, _ := db.Prepare(dbSql)
	_, err := stmt.Exec(STATUS_FAILED, orderID)
	if err != nil {
		fmt.Fprintf(w, "UPDATE ERROR: %v \n", err)
		return
	}
	fmt.Fprintf(w, "同意がキャンセルされました\n")
	chargeID, err := getChargeID(orderID, db)
	if err != nil {
		fmt.Fprintf(w, "charge ID ERR: %v\n", err)
	}
	//仮売上をキャンセル
	rawjson, err := webpayCancelProvisionalSale(chargeID, r)
	fmt.Fprintf(w, "rawjson: %v \n err: %v \n", rawjson, err)
}

/**
 * オーダーのキャンセル(借りる側)
 * @param {[type]} w http.ResponseWriter [description]
 * @param {[type]} r *http.Request       [description]
 */
func CanselOrder(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	db := dbase.OpenDB()
	var dbSql string
	orderID := r.Form.Get(ORDER_ID)
	res, err := canCancelFree(orderID, db)
	if err != nil {
		fmt.Fprintf(w, "借りれるかエラー%v \n", err)
		return
	}
	fmt.Fprintf(w, "レンタル日: %v \n ERR: %v \n", res, err)
	//キャンセルをtrueに
	if res {
		//キャンセル料が書かなとき
		t := time.Now()
		dbSql = fmt.Sprintf("UPDATE %v SET %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_ID)
		stmt, err := db.Prepare(dbSql)
		if err != nil {
			fmt.Fprintf(w, "str: %v \nstmt ERR: %v \n", dbSql, err)
			return
		}
		_, err = stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_FREE, orderID)
		if err != nil {
			fmt.Fprintf(w, "実行のエラー: %v \n ", err)
			return
		}
		chID, err := getChargeID(orderID, db)
		if err != nil {
			fmt.Fprintf(w, "chargeID: %v \n ", err)
			return
		}
		res, err := webpayCancelProvisionalSale(chID, r)
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_STATUS, ORDER_ID)
		stmt, _ = db.Prepare(dbSql)
		_, err = stmt.Exec(STATUS_FINISH, orderID)
		if err != nil {
			fmt.Fprintf(w, "最終ステータスの変更: %v \n ", err)
			return
		}
		fmt.Fprintf(w, "仮売上の無効化: %v \n もしくはエラー: %v \n", res, err)
	} else {
		//キャンセル料がかかるとき
		t := time.Now()
		dbSql = fmt.Sprintf("UPDATE %v SET %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_ID)
		stmt, _ := db.Prepare(dbSql)
		_, err := stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_PAID, orderID)
		if err != nil {
			fmt.Fprintf(w, "実行のエラー: %v \n ", err)
			return
		}
		chID, err := getChargeID(orderID, db)
		amount, err := getAmount(orderID, db)
		var amountFloat float64 = CANCEL_RATE * float64(amount)
		amount = int(amountFloat)
		res, err := webpayProvisionalToReal(chID, strconv.Itoa(amount), r)
		if err != nil {
			return
		}
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_STATUS, ORDER_ID)
		stmt, _ = db.Prepare(dbSql)
		_, err = stmt.Exec(STATUS_FINISH, orderID)
		if err != nil {
			fmt.Fprintf(w, "最終ステータスの変更: %v \n ", err)
			return
		}
		fmt.Fprintf(w, "キャンセル料: %v 円 : %v　円 \n 一部の実売上か: %v \n もしくはエラー: %v \n", amount, amountFloat, res, err)
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
	orderID := r.Form.Get(ORDER_ID)
	isCancelStr := r.Form.Get(IS_CANCEL)
	isCancel, _ := strconv.Atoi(isCancelStr)
	is, err := checkCanDelayCancelDay(orderID, db)
	if !is || err != nil {
		fmt.Fprintf(w, "できません.\nERR: %v\n", err)
		return
	}
	fmt.Fprintf(w, "キャンセル: %v\n", isCancel)
	if isCancel != 0 {
		t := time.Now()
		dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_ID)
		stmt, _ := db.Prepare(dbSql)
		_, err := stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_DELAY, orderID)
		if err != nil {
			fmt.Fprintf(w, "実行のエラー: %v \n ", err)
			return
		}
		chID, err := getChargeID(orderID, db)
		if err != nil {
			fmt.Fprintf(w, "chargeID: %v \n ", err)
			return
		}
		res, err := webpayCancelProvisionalSale(chID, r)
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_STATUS, ORDER_ID)
		stmt, _ = db.Prepare(dbSql)
		_, err = stmt.Exec(STATUS_FINISH, orderID)
		if err != nil {
			fmt.Fprintf(w, "最終ステータスの変更: %v \n ", err)
			return
		}
		fmt.Fprintf(w, "仮売上の無効化: %v \n もしくはエラー: %v \n", res, err)
		return
	} else {
		t := time.Now()
		dbSql := fmt.Sprintf("UPDATE %v SET %v=?, %v=? WHERE %v=?", ORDER, ORDER_CANCEL_DATE, ORDER_CANCEL_STATE, ORDER_ID)
		stmt, _ := db.Prepare(dbSql)
		_, err := stmt.Exec(t.Format("2006-01-02 15:04:05"), ORDER_STATE_CANCEL_DELAY, orderID)
		if err != nil {
			fmt.Fprintf(w, "実行のエラー: %v \n ", err)
			return
		}
		chID, err := getChargeID(orderID, db)
		amount, err := getAmount(orderID, db)
		if err != nil {
			fmt.Fprintf(w, "chargeID: %v \n ", err)
			return
		}
		res, err := webpayProvisionalToReal(chID, strconv.Itoa(amount), r)
		dbSql = fmt.Sprintf("UPDATE %v SET %v=? WHERE %v=?", ORDER, ORDER_STATUS, ORDER_ID)
		stmt, _ = db.Prepare(dbSql)
		_, err = stmt.Exec(STATUS_DELAY_CANCEL_REAL_SALE, orderID)
		if err != nil {
			fmt.Fprintf(w, "最終ステータスの変更: %v \n ", err)
			return
		}
		fmt.Fprintf(w, "仮売上の実売上: %v \n もしくはエラー: %v \n", res, err)
		return
	}
}

/**
 * オーダーの仮売上を実売上に
 */
func ProvisionOrderToReal() {
	//
}

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
	orderID := r.Form.Get(ORDER_ID)
	iOrderId, _ := strconv.Atoi(orderID)
	itemID, _ := getItemID(orderID, db)
	item, _ := getItemData(itemID, db)
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
				updateDepositState(orderID, DEPOSIT_STATE_FINISH, db)
				fmt.Fprintln(w, "デポジットを終了しました")
			}

		}
	}
	fmt.Fprintf(w, "オーダー: %v \n エラー: %v \n", order, err)
}

/**
 * リクエストで送られたデータのチェック
 * @return {[type]} [description]
 */
func checkRequestData() {
	//
}

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
	if subDays <= CANCEL_FREE_DAY_LIMIT {
		return false, nil
	}
	return true, nil
}

/**
 * ステータスのチェック
 * @return {[type]} [description]
 */
func checkStatus() {
}

/**
 * かせるかどうかの日にち判定
 */
func checkRentalDay(from, to time.Time, itemID string, db *sql.DB) bool {
	var able bool
	if able = checkRentalProvisonLimit(from, to); !able {
		return able
	}

	if able = checkDoubleBooking(from, to, itemID, db); !able {
		return able
	}
	return true
}

func checkRentalProvisonLimit(from, to time.Time) bool {
	nowTime := time.Now()
	nowTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), nowTime.Hour(), nowTime.Minute(), 0, 0, time.UTC)
	//契約できる日かどうか(今はとりあえず仮売上の日にちを超えないようになってるかどうか)
	subDays := calcSubDate(nowTime, from)
	if subDays > DAY_LIMIT {
		return false
	}
	return true
}

//その日にもう借りられてないかどうか
func checkDoubleBooking(tFrom, tTo time.Time, itemID string, db *sql.DB) bool {
	//始まりか終わりどちらかが利用期間にかかってる
	var count int = 0
	from := timeToStrYMD(tFrom)
	to := timeToStrYMD(tTo)
	dbSql := fmt.Sprintf("SELECT count(*) FROM %v WHERE %v=%v AND '%v' BETWEEN %v AND %v OR '%v' BETWEEN %v AND %v", ORDER, ITEM_ID, itemID, from, RENTAL_FROM, RENTAL_TO, to, RENTAL_FROM, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err := db.Query(dbSql)
	var count1 int
	if err != nil {
		return false
	}
	for res.Next() {
		if err := res.Scan(&count1); err != nil {
			return false
		}
	}
	count += count1
	//レンタルする間に他のレンタルがある場合
	dbSql = fmt.Sprintf("SELECT count(*) FROM %v WHERE %v=%v AND '%v'<%v AND '%v'>%v", ORDER, ITEM_ID, itemID, from, RENTAL_FROM, to, RENTAL_TO)
	fmt.Printf("sql: %v \n", dbSql)
	res, err = db.Query(dbSql)
	var count2 int
	if err != nil {
		return false
	}
	for res.Next() {
		if err := res.Scan(&count2); err != nil {
			return false
		}
	}
	count += count2
	if count == 0 {
		return true
	}
	return false
}

/**
 * 2016-02-12 の形を崩す
 * @param  {[type]} allTime string)       (y, m, d int [description]
 * @return {[type]}         [description]
 */
func divideTime(allTime string) (y, m, d int) {
	divTime := strings.Split(allTime, "-")
	y, _ = strconv.Atoi(divTime[0])
	m, _ = strconv.Atoi(divTime[1])
	d, _ = strconv.Atoi(divTime[2])
	return
}

//time型から y-m-dの形に治す
func timeToStrYMD(t time.Time) string {
	y := t.Year()
	m := t.Month()
	d := t.Day()
	day := fmt.Sprintf("%v-%v-%v", y, m, d)
	return day
}

/**
 * 借りる日数を計算
 */
func calcSubDate(pre, post time.Time) int {
	subTime := post.Sub(pre)
	return int(subTime.Hours() / 24)
}
