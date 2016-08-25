package account

import(
    "time"
    "fmt"
    "strings"
    "strconv"
)

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

func strTimeToTime(strTime string) time.Time {
	y, m, d := divideTime(strTime)
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
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
func isExistInDB(table, column, value string){
	
}
