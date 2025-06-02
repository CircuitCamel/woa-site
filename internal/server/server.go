package server

import (
	"fmt"
	"net/http"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func StartServer(conf util.Config) {

	mux := http.NewServeMux()

	mux.HandleFunc("/", notfound)
	mux.HandleFunc("/sessions", woa.SessionHandler)
	mux.HandleFunc("/rules", woa.Rules)
	mux.HandleFunc("/characters", woa.CharacterHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./website"))))

	fmt.Printf("Server running on port: %s", conf.PORT)
	if conf.ENV == "production" {
		http.ListenAndServeTLS(":"+conf.PORT, "/etc/nginx/ssl/el7ossen.uk_cert.pem", "/etc/nginx/ssl/el7ossen.uk_key.pem", mux)
	} else {
		http.ListenAndServe(":"+conf.PORT, mux)
	}
}

func notfound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Page Not Found", 404)
		return
	}

	woa.MainPage(w, r)
}
