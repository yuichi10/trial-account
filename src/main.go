package main

import (
	"account"
	"fmt"
	"github.com/gorilla/context"
	_ "github.com/gorilla/mux"
	"net/http"
)

func main() {
	http.HandleFunc("/", helloGo)
	http.HandleFunc("/account/PublishOrder", account.PublishOrder)
	http.HandleFunc("/account/AddCustomer", account.AddCustomer)
	http.HandleFunc("/account/ConsentOrder", account.ConsentOrder)
	http.ListenAndServe(":9977", context.ClearHandler(http.DefaultServeMux))
}

func helloGo(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello GO!!!!")
}
