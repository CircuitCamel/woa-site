package rule

import (
	"net/http"
	"os"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func TableRulesHandler(w http.ResponseWriter, r *http.Request) {
	cache.TableListTmpl.ExecuteTemplate(w, "base", cache.TableRules)
}

func TableRuleDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tablerule := vars["tablerule"]

	var selected woa.Rule
	found := false
	for _, a := range cache.TableRules {
		if a.TitlePath == tablerule {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	databytes, _ := os.ReadFile(selected.Path)

	selected.Body = util.MdToHTML(databytes)

	cache.TableTmpl.ExecuteTemplate(w, "base", selected)
}
