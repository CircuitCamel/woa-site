// Package sheet serves the character sheets. Every rule and calculation lives in
// internal/sheet; these handlers only read forms and render results.
package sheet

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

const (
	searchLimit = 40
	rollsShown  = 100
	textLimit   = 20000
)

// Tabs in display order.
var Tabs = []Tab{
	{"actions", "Actions"},
	{"spells", "Spells"},
	{"inventory", "Inventory"},
	{"features", "Features"},
	{"background", "Background"},
	{"notes", "Notes"},
	{"rolls", "Rolls"},
}

type Tab struct {
	Key   string
	Label string
}

func validTab(t string) string {
	for _, tab := range Tabs {
		if tab.Key == t {
			return t
		}
	}
	return "actions"
}

// Handler serves everything under /sheets.
type Handler struct {
	Store       *game.Store
	Auth        *auth.Service
	listTmpl    *template.Template
	builderTmpl *template.Template
	sheetTmpl   *template.Template
	editTmpl    *template.Template
	levelTmpl   *template.Template
}

func New(store *game.Store, a *auth.Service) *Handler {
	return &Handler{
		Store:       store,
		Auth:        a,
		listTmpl:    web.Parse("static/sheets/index.html"),
		builderTmpl: web.Parse("static/sheets/new.html"),
		sheetTmpl:   web.Parse("static/sheets/sheet.html", "static/sheets/panels.html"),
		editTmpl:    web.Parse("static/sheets/edit.html"),
		levelTmpl:   web.Parse("static/sheets/levelup.html"),
	}
}

// Register adds the routes. Action routes share one wrapper that handles access,
// the transaction, errors and the response.
func (h *Handler) Register(r *mux.Router) {
	page := func(path string, fn http.HandlerFunc, methods ...string) {
		r.HandleFunc(path, web.Secure(h.Auth.RequireLogin(fn))).Methods(methods...)
	}
	const id = "/sheets/{id:[0-9]+}"

	page("/sheets", h.list, http.MethodGet)
	page("/sheets/new", h.builder, http.MethodGet)
	page("/sheets/new", h.create, http.MethodPost)
	page("/sheets/new/preview", h.preview, http.MethodPost)
	page("/sheets/new/roll", h.rollScores, http.MethodPost)
	page("/sheets/compendium/spells", h.spellSearch, http.MethodGet)
	page("/sheets/compendium/items", h.itemSearch, http.MethodGet)

	page(id, h.show, http.MethodGet)
	page(id+"/edit", h.editForm, http.MethodGet)
	page(id+"/edit", h.edit, http.MethodPost)
	page(id+"/levelup", h.levelForm, http.MethodGet)
	page(id+"/levelup", h.levelUp, http.MethodPost)
	page(id+"/delete", h.remove, http.MethodPost)
	page(id+"/text", h.saveText, http.MethodPost)

	for path, fn := range map[string]mutator{
		"/hp":             doHP,
		"/inspiration":    doInspiration,
		"/exhaustion":     doExhaustion,
		"/condition":      doCondition,
		"/death":          doDeath,
		"/rest":           doRest,
		"/hitdie":         doHitDie,
		"/slot":           doSlot,
		"/pact":           doPact,
		"/resource":       doResource,
		"/roll":           doRoll,
		"/currency":       doCurrency,
		"/spell/cast":     doCast,
		"/spell/add":      doSpellAdd,
		"/spell/custom":   doSpellCustom,
		"/spell/prepare":  doSpellPrepare,
		"/spell/delete":   doSpellDelete,
		"/item/add":       doItemAdd,
		"/item/custom":    doItemCustom,
		"/item/edit":      doItemEdit,
		"/item/equip":     doItemEquip,
		"/item/attune":    doItemAttune,
		"/item/delete":    doItemDelete,
		"/feature/add":    doFeatureAdd,
		"/feature/use":    doFeatureUse,
		"/feature/delete": doFeatureDelete,
	} {
		page(id+path, h.action(fn), http.MethodPost)
	}
}

// userError is a message safe to show. Any other error is treated as internal,
// so database errors never reach the page.
type userError string

func (e userError) Error() string { return string(e) }

func fail(msg string) error { return userError(msg) }

func failErr(err error) error {
	if err == nil {
		return nil
	}
	return userError(err.Error())
}

func access(r *http.Request) game.Access {
	u := auth.CurrentUser(r)
	return game.Access{UserID: u.ID, IsAdmin: u.IsAdmin()}
}

func charID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	return id, err == nil
}

func isPartial(r *http.Request) bool { return r.Header.Get("X-Partial") == "1" }

// internal logs an unexpected error and shows the generic error page.
func internal(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, game.ErrNotFound) {
		util.ErrPage(w, r, 404)
		return
	}
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	util.ErrPage(w, r, 500)
}

type mutator func(*game.Mut, *http.Request) error

// action runs a mutator in a transaction. Success redirects back to the tab, or
// returns the re-rendered sheet to client JS. A user error re-renders the sheet
// with the message and nothing saved.
func (h *Handler) action(fn mutator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := charID(r)
		if !ok {
			util.ErrPage(w, r, 404)
			return
		}
		tab := validTab(r.PostFormValue("tab"))
		err := h.Store.Update(id, access(r), func(m *game.Mut) error { return fn(m, r) })
		var ue userError
		switch {
		case errors.As(err, &ue):
			h.renderSheet(w, r, id, tab, string(ue), http.StatusBadRequest)
		case err != nil:
			internal(w, r, err)
		case isPartial(r):
			h.renderSheet(w, r, id, tab, "", http.StatusOK)
		default:
			http.Redirect(w, r, "/sheets/"+strconv.FormatInt(id, 10)+"?tab="+tab, http.StatusSeeOther)
		}
	}
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	id, ok := charID(r)
	if !ok {
		util.ErrPage(w, r, 404)
		return
	}
	h.renderSheet(w, r, id, validTab(r.URL.Query().Get("tab")), "", http.StatusOK)
}

func (h *Handler) renderSheet(w http.ResponseWriter, r *http.Request, id int64, tab, errMsg string, status int) {
	c, err := h.Store.Get(id, access(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	page, err := h.buildPage(r, c, tab, errMsg)
	if err != nil {
		internal(w, r, err)
		return
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	if isPartial(r) {
		web.Render(w, h.sheetTmpl, "sheet-body", page)
		return
	}
	web.Render(w, h.sheetTmpl, "base", page)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.CurrentUser(r)
	rows, err := h.Store.List(u.ID)
	if err != nil {
		internal(w, r, err)
		return
	}
	web.Render(w, h.listTmpl, "base", struct {
		web.Base
		Characters []game.Summary
	}{web.Base{User: u}, rows})
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := charID(r)
	if !ok {
		util.ErrPage(w, r, 404)
		return
	}
	if r.PostFormValue("confirm") != "yes" {
		h.renderSheet(w, r, id, "notes", "Tick the box to confirm deleting this character.", http.StatusBadRequest)
		return
	}
	if err := h.Store.Delete(id, access(r)); err != nil {
		internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/sheets", http.StatusSeeOther)
}

// saveText stores free-text fields. Only fields present in the form change, so
// each tab's form and the autosave can send just their own.
func (h *Handler) saveText(w http.ResponseWriter, r *http.Request) {
	id, ok := charID(r)
	if !ok {
		util.ErrPage(w, r, 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	tab := validTab(r.PostFormValue("tab"))
	err := h.Store.Update(id, access(r), func(m *game.Mut) error {
		for name, field := range textFields(m.C) {
			if vals, present := r.PostForm[name]; present {
				if len(vals[0]) > textLimit {
					return fail("That text is too long.")
				}
				*field = strings.ReplaceAll(vals[0], "\r\n", "\n")
			}
		}
		return nil
	})
	var ue userError
	autosave := r.Header.Get("X-Autosave") == "1"
	switch {
	case errors.As(err, &ue) && autosave:
		http.Error(w, string(ue), http.StatusBadRequest)
	case errors.As(err, &ue):
		h.renderSheet(w, r, id, tab, string(ue), http.StatusBadRequest)
	case err != nil:
		internal(w, r, err)
	case autosave:
		w.WriteHeader(http.StatusNoContent)
	case isPartial(r):
		h.renderSheet(w, r, id, tab, "", http.StatusOK)
	default:
		http.Redirect(w, r, "/sheets/"+strconv.FormatInt(id, 10)+"?tab="+tab, http.StatusSeeOther)
	}
}

// textFields whitelists the free-text columns a form may set.
func textFields(c *game.Character) map[string]*string {
	return map[string]*string{
		"age": &c.Age, "height": &c.Height, "weight_text": &c.WeightText, "eyes": &c.Eyes, "skin": &c.Skin, "hair": &c.Hair, "faith": &c.Faith,
		"personality": &c.Personality, "ideals": &c.Ideals, "bonds": &c.Bonds, "flaws": &c.Flaws,
		"appearance": &c.Appearance, "backstory": &c.Backstory, "allies": &c.Allies, "notes": &c.Notes,
	}
}

func formInt(r *http.Request, name string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(name)))
	if err != nil {
		return def
	}
	return v
}

func formID(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.PostFormValue(name), 10, 64)
	return v
}

func clampInt(n, lo, hi int) int {
	return max(lo, min(n, hi))
}

// clip limits one-line text and trims it.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = strings.ToValidUTF8(s[:n], "")
	}
	return s
}
