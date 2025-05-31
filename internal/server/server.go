package server

import (
	"fmt"
	"net/http"
	woa "warofages/internal/woa/mainpage"
)

func StartServer(port string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", notfound)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./website"))))

	fmt.Printf("Server running on port: %s", port)
	http.ListenAndServe(":"+port, mux)
}

func notfound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Page Not Found", 404)
		return
	}

	woa.MainPage(w, r)
}
