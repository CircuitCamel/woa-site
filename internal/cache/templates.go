package cache

import "text/template"

func LoadAllTemplates() error {
	var err error
	CharListTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/characters/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	CharTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/characters/character.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	SessionListTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/sessions/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	SessionTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/sessions/session.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	RulesTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	MechanicListTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/mechanics/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	MechanicTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/mechanics/mechanic.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	TableListTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/table/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	TableTmpl, err = template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/table/rule.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return err
	}
	return nil
}
