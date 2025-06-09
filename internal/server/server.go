package server

import (
	"fmt"
	"net/http"
	"warofages/internal/util"
	"warofages/internal/woa/character"
	"warofages/internal/woa/landing"
	"warofages/internal/woa/rule"
	"warofages/internal/woa/session"

	"github.com/gorilla/mux"
)

func StartServer(conf util.Config) {

	mux := mux.NewRouter()

	mux.HandleFunc("/", notfound)
	mux.HandleFunc("/sessions", session.SessionHandler)
	mux.HandleFunc("/rules", rule.RulesHandler)
	mux.HandleFunc("/characters", character.CharactersHandler)
	mux.HandleFunc("/characters/{name}", character.CharacterDetailHandler)

	mux.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))),
	)

	fmt.Printf("Server running on port: %s", conf.PORT)
	if conf.ENV == "production" {
		http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, mux)
	} else if conf.ENV == "staging" {
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
