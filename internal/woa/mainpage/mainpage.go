package woa

import (
	"net/http"
)

func MainPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "website/index.html")
}
