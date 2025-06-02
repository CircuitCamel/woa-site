package server

import (
	"fmt"
	"net/http"
	"warofages/internal/woa"
)

func StartServer(port string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", notfound)
	mux.HandleFunc("/sessions", woa.SessionHandler)
	mux.HandleFunc("/rules", woa.Rules)
	mux.HandleFunc("/characters", woa.CharacterHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./website"))))

	fmt.Printf("Server running on port: %s", port)
	http.ListenAndServeTLS(":"+port, "/etc/nginx/ssl/el7ossen.uk_cert.pem", "/etc/nginx/ssl/el7ossen.uk_key.pem", mux)
}

func notfound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Page Not Found", 404)
		return
	}

	woa.MainPage(w, r)
}
