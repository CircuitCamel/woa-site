package rule

import (
	"net/http"
	"warofages/internal/cache"
)

func RulesHandler(w http.ResponseWriter, r *http.Request) {
	cache.RulesTmpl.ExecuteTemplate(w, "base", nil)
}
