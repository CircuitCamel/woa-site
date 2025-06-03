package server

import (
	"fmt"
	"net/http"
	"warofages/internal/util"
	"warofages/internal/woa/character"
	landing "warofages/internal/woa/mainpage"
	"warofages/internal/woa/rule"
	"warofages/internal/woa/session"
)

func StartServer(conf util.Config) {

	mux := http.NewServeMux()

	mux.HandleFunc("/", notfound)
	mux.HandleFunc("/sessions", session.SessionHandler)
	mux.HandleFunc("/rules", rule.RulesHandler)
	mux.HandleFunc("/characters", character.CharacterHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	fmt.Printf("Server running on port: %s", conf.PORT)
	if conf.ENV == "production" {
		http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, mux)
	} else {
		http.ListenAndServe(":"+conf.PORT, mux)
	}
}

func notfound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Page Not Found", 404)
		return
	}

	landing.Index(w, r)
}
