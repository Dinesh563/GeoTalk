package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
)

func main() {
	fmt.Println("Running GeoTalk")

	r := mux.NewRouter()

	// Serve static assets under /static/
	staticDir := http.Dir("./static/")
	fs := http.StripPrefix("/static/", http.FileServer(staticDir))
	r.PathPrefix("/static/").Handler(fs)

	// Serve index.html on root GET
	r.HandleFunc("/", HandleHomeRoute).Methods("GET")

	r.HandleFunc("/PutMessage", PutMessage).Methods("POST")

	r.HandleFunc("/msgs", GetMessages).Methods("GET")

	log.Println("Running HTTPS server on :8443")
	err := http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", r)
	if err != nil {
		log.Fatal(err)
	}
}

func HandleHomeRoute(w http.ResponseWriter, r *http.Request) {
	tmplPath := filepath.Join("static", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}
