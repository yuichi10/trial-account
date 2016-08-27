package main

import (
	"account"
	"fmt"
	"github.com/gorilla/context"
	_ "github.com/gorilla/mux"
	"net/http"
	"github.com/joho/godotenv"
	"log"
)

func Env_load() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }
}

func main() {
	Env_load()
	http.HandleFunc("/", helloGo)
	http.HandleFunc("/account/testDB", account.TestDB)
	http.HandleFunc("/account/publishOrder", account.PublishOrder)
	http.HandleFunc("/account/addCustomer", account.AddCustomer)
	http.HandleFunc("/account/consentOrder", account.ConsentOrder)
	http.HandleFunc("/account/disagreeOrder", account.DisagreeOrder)
	http.HandleFunc("/account/canselOrder", account.CanselOrder)
	http.HandleFunc("/account/delayCanselReport", account.DelayCanselReport)
	//http.HandleFunc("/account/startNegotiateDeposit", account.StartNegotiateDeposit)
	//http.HandleFunc("/account/updateDeposit", account.UpdateDeposit)
	//http.HandleFunc("/account/consentDeposit", account.ConsentDeposit)
	http.ListenAndServe(":9977", context.ClearHandler(http.DefaultServeMux))
}

func helloGo(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello GO!!!!")
}
