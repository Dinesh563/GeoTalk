package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"text/template"
	"github.com/gorilla/mux"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

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

	// Apply CORS middleware
	handlerWithCORS := withCORS(r)

	log.Println("Running HTTP server on :8443")
	err := http.ListenAndServe(":8443", handlerWithCORS)

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
