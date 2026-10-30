package cache

import (
	"text/template"
	"warofages/internal/woa"
)

var (
	Characters       []woa.Character
	Sessions         []woa.Session
	Mechanics        []woa.Rule
	TableRules       []woa.Rule
	NPCs             []woa.NPC
	Items            []woa.Item
	CharListTmpl     *template.Template
	CharTmpl         *template.Template
	SessionListTmpl  *template.Template
	SessionTmpl      *template.Template
	RulesTmpl        *template.Template
	MechanicListTmpl *template.Template
	MechanicTmpl     *template.Template
	TableListTmpl    *template.Template
	TableTmpl        *template.Template
	NPCListTmpl      *template.Template
	NPCTmpl          *template.Template
	ItemListTmpl     *template.Template
	ItemTmpl         *template.Template
)

func LoadAll() {
	var err error

	Characters, err = getCharacters()
	if err != nil {
		return
	}
	Sessions, err = getSessions()
	if err != nil {
		return
	}
	Mechanics, err = getMechanics()
	if err != nil {
		return
	}
	TableRules, err = getTableRules()
	if err != nil {
		return
	}
	NPCs, err = getNPCs()
	if err != nil {
		return
	}
	Items, err = getItems()
	if err != nil {
		return
	}
	err = LoadAllTemplates()
	if err != nil {
		return
	}
}
