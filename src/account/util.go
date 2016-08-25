package account

import(
    "time"
    "fmt"
    "strings"
    "strconv"
	"database/sql"
)

/**
 * 2016-02-12 の形を崩す
 * @param  {[type]} allTime string)       (y, m, d int [description]
 * @return {[type]}         [description]
 */
func divideTime(allTime string) (y, m, d int, err error) {
	divTime := strings.Split(allTime, "-")
	y, err = strconv.Atoi(divTime[0])
	if err != nil {
		y, m, d = 0, 0, 0
		return
	}
	m, err = strconv.Atoi(divTime[1])
	if err != nil {
		y, m, d = 0, 0, 0
		return
	}
	d, err = strconv.Atoi(divTime[2])
	if err != nil {
		y, m, d = 0, 0, 0
		return 
	}
	return
}

func checkStrTime(strTime string) bool {
	_, err := strTimeToTime(strTime)
	if err != nil {
		return false
	}
	return true
}

//%v-%v-%v　のtimeを　time.Timeに変換する
func strTimeToTime(strTime string) (time.Time, error ){
	y, m, d, err := divideTime(strTime)
	if err != nil {
		return time.Now(), fmt.Errorf("間違った数字が呼ばれました")
	}
	date := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if date.Year() == y && date.Month() == time.Month(m) && date.Day() == d {
        return date, nil
    }
    return time.Now(), fmt.Errorf("%d-%d-%d is not exist", y, m, d)
}

//time型から y-m-dの形に治す
func timeToStrYMD(t time.Time) string {
	y := t.Year()
	m := int(t.Month())
	d := t.Day()
	day := fmt.Sprintf("%v-%v-%v", y, m, d)
	return day
}

func timeToTimeYMD(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

/**
 * 借りる日数を計算
 */
func calcSubDate(pre, post time.Time) int {
	subTime := post.Sub(pre)
	days := int(subTime.Hours()) / 24
	if subTime.Minutes()/(24*60)-float64(days) > 0 {
		days++
	}
	return days
}

//一つのカラムで存在するかどうかのチェック
func countColumnInDB(table, column, value string, db *sql.DB) int {
	dbSql := fmt.Sprintf("SELECT count(*) FROM %v WHERE %v=%v", table, column, value)
	res, err := db.Query(dbSql)
	if err != nil {
		return 0
	}
	var count int
	for res.Next() {
		if err := res.Scan(&count); err != nil {
			return 0
		}
	}
	return count
}

//一つだけ存在するかどうか
func isExitInDBUnique(table, column, value string, db *sql.DB) bool {
	count := countColumnInDB(table, column, value, db)
	if count != 1 {
		return false
	}
	return true
}