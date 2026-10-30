// Package admin serves the dashboard the DM uses to manage players.
package admin

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"warofages/internal/auth"
	game "warofages/internal/sheet"
	"warofages/internal/util"
	"warofages/internal/woa/web"

	"github.com/gorilla/mux"
)

type Handler struct {
	Auth       *auth.Service
	Store      *game.Store
	dashTmpl   *template.Template
	playerTmpl *template.Template
}

func New(a *auth.Service, store *game.Store) *Handler {
	return &Handler{
		Auth:       a,
		Store:      store,
		dashTmpl:   web.Parse("static/admin/index.html"),
		playerTmpl: web.Parse("static/admin/player.html"),
	}
}

func notFound(w http.ResponseWriter, r *http.Request) { util.ErrPage(w, r, 404) }

// Register adds the routes, all behind the admin check.
func (h *Handler) Register(r *mux.Router) {
	route := func(path string, fn http.HandlerFunc, method string) {
		r.HandleFunc(path, web.Secure(h.Auth.RequireAdmin(notFound, fn))).Methods(method)
	}
	const player = "/admin/players/{id:[0-9]+}"
	route("/admin", h.dashboard, http.MethodGet)
	route("/admin/players", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}, http.MethodGet)
	route("/admin/players", h.create, http.MethodPost)
	route(player, h.player, http.MethodGet)
	route(player+"/reset", h.reset, http.MethodPost)
	route(player+"/disable", h.setDisabled(true), http.MethodPost)
	route(player+"/enable", h.setDisabled(false), http.MethodPost)
	route(player+"/delete", h.remove, http.MethodPost)
}

type dashboardPage struct {
	web.Base
	Users      []auth.User
	Characters []game.Summary
	Created    string
	TempPass   string
	Username   string
	Role       string
}

func (h *Handler) renderDashboard(w http.ResponseWriter, r *http.Request, page dashboardPage) {
	var err error
	page.User = auth.CurrentUser(r)
	if page.Users, err = h.Auth.ListUsers(); err == nil {
		page.Characters, err = h.Store.List(0)
	}
	if err != nil {
		log.Printf("admin dashboard: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	web.Render(w, h.dashTmpl, "base", page)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	h.renderDashboard(w, r, dashboardPage{})
}

// create adds an account. The temporary password is shown once on this response
// and never put in a URL.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	username := auth.NormalizeUsername(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	role := r.PostFormValue("role")
	generated := password == ""
	if generated {
		password = auth.GeneratePassword()
	}
	_, err := h.Auth.CreateUser(username, password, role)
	if err != nil {
		if !isUserError(err) {
			log.Printf("create user: %v", err)
			util.ErrPage(w, r, 500)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		page := dashboardPage{Username: username, Role: role}
		page.Error = err.Error()
		h.renderDashboard(w, r, page)
		return
	}
	page := dashboardPage{Created: username}
	if generated {
		page.TempPass = password
	}
	h.renderDashboard(w, r, page)
}

func isUserError(err error) bool {
	return errors.Is(err, auth.ErrUsernameInvalid) || errors.Is(err, auth.ErrUsernameTaken) ||
		errors.Is(err, auth.ErrPasswordInvalid) || errors.Is(err, auth.ErrLastAdmin) || strings.Contains(err.Error(), "unknown role")
}

func userID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	return id
}

type playerPage struct {
	web.Base
	Player     *auth.User
	Characters []game.Summary
	TempPass   string
	IsSelf     bool
}

func (h *Handler) renderPlayer(w http.ResponseWriter, r *http.Request, page playerPage) {
	id := userID(r)
	p, err := h.Auth.GetUser(id)
	if errors.Is(err, auth.ErrNotFound) {
		util.ErrPage(w, r, 404)
		return
	}
	if err == nil {
		page.Characters, err = h.Store.List(id)
	}
	if err != nil {
		log.Printf("admin player: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	page.User = auth.CurrentUser(r)
	page.Player = p
	page.IsSelf = p.ID == page.User.ID
	web.Render(w, h.playerTmpl, "base", page)
}

func (h *Handler) player(w http.ResponseWriter, r *http.Request) {
	h.renderPlayer(w, r, playerPage{})
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	id := userID(r)
	password := r.PostFormValue("password")
	generated := password == ""
	if generated {
		password = auth.GeneratePassword()
	}
	page := playerPage{}
	switch err := h.Auth.ResetPassword(id, password); {
	case errors.Is(err, auth.ErrNotFound):
		util.ErrPage(w, r, 404)
		return
	case errors.Is(err, auth.ErrPasswordInvalid):
		w.WriteHeader(http.StatusBadRequest)
		page.Error = err.Error()
	case err != nil:
		log.Printf("reset password: %v", err)
		util.ErrPage(w, r, 500)
		return
	default:
		page.Info = "Password reset. They'll have to change it when they next log in."
		if generated {
			page.TempPass = password
		}
	}
	h.renderPlayer(w, r, page)
}

func (h *Handler) setDisabled(disabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := userID(r)
		page := playerPage{}
		if id == auth.CurrentUser(r).ID {
			w.WriteHeader(http.StatusBadRequest)
			page.Error = "You can't disable your own account."
			h.renderPlayer(w, r, page)
			return
		}
		switch err := h.Auth.SetDisabled(id, disabled); {
		case errors.Is(err, auth.ErrNotFound):
			util.ErrPage(w, r, 404)
			return
		case errors.Is(err, auth.ErrLastAdmin):
			w.WriteHeader(http.StatusBadRequest)
			page.Error = err.Error()
			h.renderPlayer(w, r, page)
			return
		case err != nil:
			log.Printf("set disabled: %v", err)
			util.ErrPage(w, r, 500)
			return
		}
		http.Redirect(w, r, "/admin/players/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	}
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id := userID(r)
	page := playerPage{}
	if id == auth.CurrentUser(r).ID {
		w.WriteHeader(http.StatusBadRequest)
		page.Error = "You can't delete your own account."
		h.renderPlayer(w, r, page)
		return
	}
	if r.PostFormValue("confirm") != "yes" {
		w.WriteHeader(http.StatusBadRequest)
		page.Error = "Tick the box to confirm deleting this account and its characters."
		h.renderPlayer(w, r, page)
		return
	}
	switch err := h.Auth.DeleteUser(id); {
	case errors.Is(err, auth.ErrNotFound):
		util.ErrPage(w, r, 404)
		return
	case errors.Is(err, auth.ErrLastAdmin):
		w.WriteHeader(http.StatusBadRequest)
		page.Error = err.Error()
		h.renderPlayer(w, r, page)
		return
	case err != nil:
		log.Printf("delete user: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
