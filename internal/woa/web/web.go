// Package web holds what the login-only pages share: templates and headers.
package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"warofages/internal/auth"
)

// Base is embedded in every login-only page's data for the navigation bar.
type Base struct {
	User  *auth.User
	Error string
	Info  string
}

var funcs = template.FuncMap{
	"signed": func(n int) string {
		if n >= 0 {
			return "+" + strconv.Itoa(n)
		}
		return strconv.Itoa(n)
	},
	"dict": func(pairs ...any) (map[string]any, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict needs key and value pairs")
		}
		out := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			out[key] = pairs[i+1]
		}
		return out, nil
	},
	"sub":   func(a, b int) int { return a - b },
	"upper": strings.ToUpper,
	"date":  func(t time.Time) string { return t.Format("2 Jan 2006") },
	"unix":  func(n int64) string { return time.Unix(n, 0).Format("2 Jan 15:04") },
	"seq": func(n int) []int {
		out := make([]int, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, i)
		}
		return out
	},
	"pounds":  func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) },
	"ordinal": Ordinal,
}

// Ordinal formats 1 as 1st, 2 as 2nd and so on.
func Ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", n)
}

// Parse loads a page with the shared layout. It uses html/template, unlike the
// public content pages, because these pages render what players type.
func Parse(pages ...string) *template.Template {
	files := []string{
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/templates/footer.html",
		"static/templates/authnav.html",
	}
	return template.Must(template.New("").Funcs(funcs).ParseFiles(append(files, pages...)...))
}

// Render executes a template into a buffer first, so a template error gives a
// clean 500 rather than half a page.
func Render(w http.ResponseWriter, t *template.Template, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

// Secure adds the headers every login-only page sends. The policy blocks inline
// scripts and event handlers; styles are left alone because the shared error
// pages use an inline <style>.
func Secure(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next(w, r)
	}
}
