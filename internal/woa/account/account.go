// Package account serves login, logout and the password page.
package account

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"warofages/internal/auth"
	"warofages/internal/util"
	"warofages/internal/woa/web"
)

type Handler struct {
	Auth        *auth.Service
	loginTmpl   *template.Template
	accountTmpl *template.Template
}

func New(a *auth.Service) *Handler {
	return &Handler{
		Auth:        a,
		loginTmpl:   web.Parse("static/account/login.html"),
		accountTmpl: web.Parse("static/account/index.html"),
	}
}

type loginPage struct {
	web.Base
	Next     string
	Username string
}

func (h *Handler) LoginForm(w http.ResponseWriter, r *http.Request) {
	if h.Auth.UserFromRequest(r) != nil {
		http.Redirect(w, r, auth.SafeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	web.Render(w, h.loginTmpl, "base", loginPage{Next: auth.SafeNext(r.URL.Query().Get("next"))})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	username := auth.NormalizeUsername(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	next := auth.SafeNext(r.PostFormValue("next"))
	page := loginPage{Next: next, Username: username}

	if h.Auth.Limiter.Blocked(r, username) {
		page.Error = "Too many failed attempts. Try again in 15 minutes."
		w.WriteHeader(http.StatusTooManyRequests)
		web.Render(w, h.loginTmpl, "base", page)
		return
	}
	u, err := h.Auth.Authenticate(username, password)
	if errors.Is(err, auth.ErrInvalidLogin) {
		h.Auth.Limiter.Fail(r, username)
		page.Error = "Invalid username or password."
		w.WriteHeader(http.StatusUnauthorized)
		web.Render(w, h.loginTmpl, "base", page)
		return
	}
	if err != nil {
		log.Printf("login: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	h.Auth.Limiter.Succeed(r, username)
	if err := h.Auth.StartSession(w, u.ID); err != nil {
		log.Printf("start session: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Auth.EndSession(w, r); err != nil {
		log.Printf("end session: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	page := web.Base{User: auth.CurrentUser(r)}
	if r.URL.Query().Get("changed") == "1" {
		page.Info = "Your password has been changed."
	}
	web.Render(w, h.accountTmpl, "base", page)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := auth.CurrentUser(r)
	next := r.PostFormValue("new_password")
	var err error
	if next != r.PostFormValue("confirm_password") {
		err = errors.New("the new passwords don't match")
	} else {
		err = h.Auth.ChangePassword(u.ID, r.PostFormValue("current_password"), next)
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		web.Render(w, h.accountTmpl, "base", web.Base{User: u, Error: err.Error()})
		return
	}
	if err := h.Auth.StartSession(w, u.ID); err != nil {
		log.Printf("start session: %v", err)
		util.ErrPage(w, r, 500)
		return
	}
	http.Redirect(w, r, "/account?changed=1", http.StatusSeeOther)
}
