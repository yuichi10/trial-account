package dbase

import (
    "database/sql"
  _ "github.com/go-sql-driver/mysql"
    "os"
    "fmt"
)

func OpenDB() *sql.DB {
    dbInfo := fmt.Sprintf("%v@%v/%v?charset=utf8&parseTime=True", os.Getenv("DB_USER_NAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
    db, err := sql.Open(os.Getenv("DB"), dbInfo)
    if err != nil {
        panic(err.Error())
    }
    return db
}