package util

import (
	"net/http"
	"strconv"
)

func ErrPage(w http.ResponseWriter, r *http.Request, code int) {
	w.WriteHeader(code)
	http.ServeFile(w, r, "static/errors/"+strconv.Itoa(code)+".html")
}
