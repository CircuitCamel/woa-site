package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa/character"
	"warofages/internal/woa/landing"
	"warofages/internal/woa/rule"
	"warofages/internal/woa/session"

	"github.com/gorilla/mux"
)

func StartServer(conf util.Config) {

	go cache.LoadAll()

	mux := mux.NewRouter().StrictSlash(true)

	mux.HandleFunc("/", landing.Index)
	mux.HandleFunc("/characters", character.CharactersHandler)
	mux.HandleFunc("/characters/{name}", character.CharacterDetailHandler)
	mux.HandleFunc("/sessions", session.SessionsHandler)
	mux.HandleFunc("/sessions/{session}", session.SessionDetailHandler)
	mux.HandleFunc("/rules", rule.RulesHandler)
	mux.HandleFunc("/rules/mechanics", rule.MechanicsHandler)
	mux.HandleFunc("/rules/mechanics/{mechanic}", rule.MechanicDetailHandler)
	mux.HandleFunc("/rules/table", rule.TableRulesHandler)
	mux.HandleFunc("/rules/table/{tablerule}", rule.TableRuleDetailHandler)

	mux.PathPrefix("/static").Handler(
		http.StripPrefix("/static", http.FileServer(http.Dir("./static"))),
	)
	mux.NotFoundHandler = http.HandlerFunc(notfound)

	fmt.Printf("Server running on port: %s", conf.PORT)
	switch conf.ENV {
	case "production":
		http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, lowerCaseURI(mux))
	case "staging":
		log.Fatal(http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, lowerCaseURI(mux)))
	default:
		log.Fatal(http.ListenAndServe(":"+conf.PORT, lowerCaseURI(mux)))
	}
}

func notfound(w http.ResponseWriter, r *http.Request) {
	util.ErrPage(w, r, 404)
}

func lowerCaseURI(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.ToLower(r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
