package main

import "net/http"
import "github.com/artificial-universe-maker/go-utilities/db"

func main() {
	http.HandleFunc("/", processRequest)
	http.ListenAndServe(":8080", nil)
}

func processRequest(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	project_id := r.Form.Get("project-id")

	db.InitializeDB()

	go initiateCompiler(project_id)
}

func initiateCompiler(project_id string) {
}
