package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"warofages/internal/auth"
	"warofages/internal/cache"
	"warofages/internal/db"
	game "warofages/internal/sheet"
	"warofages/internal/util"
	"warofages/internal/woa/account"
	"warofages/internal/woa/admin"
	"warofages/internal/woa/character"
	"warofages/internal/woa/item"
	"warofages/internal/woa/landing"
	"warofages/internal/woa/npc"
	"warofages/internal/woa/rule"
	"warofages/internal/woa/session"
	"warofages/internal/woa/sheet"
	"warofages/internal/woa/web"

	"github.com/gorilla/mux"
)

func StartServer(conf util.Config) {

	go cache.LoadAll()

	database, err := db.Open(conf.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	handler, err := buildHandler(conf, database)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Server running on port: %s", conf.PORT)
	switch conf.ENV {
	case "production":
		http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, handler)
	case "staging":
		log.Fatal(http.ListenAndServeTLS(":"+conf.PORT, conf.CRT, conf.KEY, handler))
	default:
		log.Fatal(http.ListenAndServe(":"+conf.PORT, handler))
	}
}

// buildHandler creates the first admin if needed and assembles all the routes.
func buildHandler(conf util.Config, database *sql.DB) (http.Handler, error) {
	secure := conf.ENV == "production" || conf.ENV == "staging"
	authService, err := auth.New(database, secure)
	if err != nil {
		return nil, err
	}
	generated, err := authService.EnsureAdmin(conf.AdminUsername, conf.AdminPassword)
	if err != nil {
		return nil, err
	}
	if generated != "" {
		fmt.Printf("Created admin account %q with temporary password: %s\nThis is shown once. You'll have to change it when you first log in.\n", conf.AdminUsername, generated)
	}

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
	mux.HandleFunc("/npcs", npc.NPCsHandler)
	mux.HandleFunc("/npcs/{npc}", npc.NPCDetailHandler)
	mux.HandleFunc("/items", item.ItemsHandler)
	mux.HandleFunc("/items/{item}", item.ItemDetailHandler)

	store := &game.Store{DB: database}
	accounts := account.New(authService)
	mux.HandleFunc("/login", web.Secure(accounts.LoginForm)).Methods(http.MethodGet)
	mux.HandleFunc("/login", web.Secure(accounts.Login)).Methods(http.MethodPost)
	mux.HandleFunc("/logout", web.Secure(accounts.Logout)).Methods(http.MethodPost)
	mux.HandleFunc("/account", web.Secure(authService.RequireLogin(accounts.Account))).Methods(http.MethodGet)
	mux.HandleFunc("/account", web.Secure(authService.RequireLogin(accounts.ChangePassword))).Methods(http.MethodPost)
	sheet.New(store, authService).Register(mux)
	admin.New(authService, store).Register(mux)

	mux.PathPrefix("/static").Handler(
		http.StripPrefix("/static", http.FileServer(http.Dir("./static"))),
	)
	mux.NotFoundHandler = http.HandlerFunc(notfound)

	return lowerCaseURI(http.NewCrossOriginProtection().Handler(mux)), nil
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
