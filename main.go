package main

import "net/http"

func main() {
	http.HandleFunc("/", processRequest)
	http.ListenAndServe(":8080", nil)
}

func processRequest(w http.ResponseWriter, r *http.Request) {
	go initiateCompiler()
}

func initiateCompiler() {

}
