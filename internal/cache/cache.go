package cache

import (
	"warofages/internal/woa"
)

var (
	Characters []woa.Character
	Sessions   []woa.Session
	Mechanics  []woa.Rule
	TableRules []woa.Rule
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
}
