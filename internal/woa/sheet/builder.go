package sheet

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"warofages/internal/auth"
	game "warofages/internal/sheet"
	"warofages/internal/srd"
	"warofages/internal/woa/web"
)

// BuilderPage is the character builder form.
type BuilderPage struct {
	web.Base
	Draft       game.Draft
	Preview     Preview
	Races       []srd.Race
	Classes     []srd.Class
	Backgrounds []srd.Background
	Alignments  []string
	Skills      []game.Skill
	Abilities   []AbilityField
	Picked      map[string]bool
	BonusPicked map[string]bool
}

type AbilityField struct {
	Key   string
	Name  string
	Score int
}

// Preview is the server-calculated summary beside the builder form.
type Preview struct {
	C          *game.Character
	D          *game.Derived
	Error      string
	PointBuy   bool
	Spent      int
	Budget     int
	Class      *srd.Class
	Race       *srd.Race
	SkillNames string
}

func readDraft(r *http.Request) game.Draft {
	d := game.Draft{
		Name:             clip(r.PostFormValue("name"), 200),
		Race:             r.PostFormValue("race"),
		RaceCustom:       clip(r.PostFormValue("race_custom"), 100),
		Class:            r.PostFormValue("class"),
		Subclass:         r.PostFormValue("subclass"),
		Level:            formInt(r, "level", 1),
		Background:       r.PostFormValue("background"),
		BackgroundCustom: clip(r.PostFormValue("background_custom"), 100),
		Alignment:        clip(r.PostFormValue("alignment"), 40),
		Method:           r.PostFormValue("method"),
		Skills:           r.PostForm["skills"],
		BonusChoices:     r.PostForm["bonus_choice"],
	}
	for i, key := range game.Abilities {
		d.Scores[i] = formInt(r, "score_"+key, 10)
	}
	return d
}

func previewFor(d game.Draft) Preview {
	p := Preview{Budget: game.PointBuyBudget, PointBuy: d.Method == "point"}
	if strings.TrimSpace(d.Name) == "" {
		d.Name = "Unnamed" // a preview shouldn't complain about the name yet
	}
	raceIndex, _, _ := strings.Cut(d.Race, "/")
	if race, ok := srd.Get().Race(raceIndex); ok {
		p.Race = race
	}
	if cl, ok := srd.Get().Class(d.Class); ok {
		p.Class = cl
		var names []string
		for _, s := range cl.SkillOptions {
			if skill, ok := game.SkillByIndex(s); ok {
				names = append(names, skill.Name)
			}
		}
		p.SkillNames = strings.Join(names, ", ")
	}
	c, spent, err := d.Build()
	p.Spent = spent
	if err != nil {
		p.Error = err.Error()
		return p
	}
	p.C, p.D = c, game.Compute(c)
	return p
}

func (h *Handler) builderPage(r *http.Request, d game.Draft, errMsg string) BuilderPage {
	page := BuilderPage{
		Base:        web.Base{User: auth.CurrentUser(r), Error: errMsg},
		Draft:       d,
		Preview:     previewFor(d),
		Races:       srd.Get().Races,
		Classes:     srd.Get().Classes,
		Backgrounds: srd.Get().Backgrounds,
		Alignments:  game.Alignments,
		Skills:      game.Skills,
		Picked:      map[string]bool{},
		BonusPicked: map[string]bool{},
	}
	for i, key := range game.Abilities {
		page.Abilities = append(page.Abilities, AbilityField{Key: key, Name: game.AbilityNames[key], Score: d.Scores[i]})
	}
	for _, s := range d.Skills {
		page.Picked[s] = true
	}
	for _, b := range d.BonusChoices {
		page.BonusPicked[b] = true
	}
	return page
}

func (h *Handler) builder(w http.ResponseWriter, r *http.Request) {
	web.Render(w, h.builderTmpl, "base", h.builderPage(r, game.DefaultDraft(), ""))
}

// preview returns the summary for client JS. It saves nothing.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	web.Render(w, h.builderTmpl, "builder-preview", previewFor(readDraft(r)))
}

// rollScores fills the form with server-rolled 4d6 scores.
func (h *Handler) rollScores(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	d := readDraft(r)
	d.Method = "manual"
	d.Scores = game.Roll4d6()
	web.Render(w, h.builderTmpl, "base", h.builderPage(r, d, ""))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	d := readDraft(r)
	c, _, err := d.Build()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		web.Render(w, h.builderTmpl, "base", h.builderPage(r, d, err.Error()))
		return
	}
	id, err := h.Store.Create(c, auth.CurrentUser(r).ID)
	if err != nil {
		internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/sheets/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// LevelPage is the level up form.
type LevelPage struct {
	web.Base
	C         *game.Character
	D         *game.Derived
	ClassLine string
	Options   []LevelOption
	Abilities []string
	Names     map[string]string
}

type LevelOption struct {
	Class      srd.Class
	Current    int
	Next       int
	Blocked    string
	HasASI     bool
	Subclasses []srd.Subclass
	Subclass   string
}

func (h *Handler) levelPage(r *http.Request, c *game.Character, errMsg string) LevelPage {
	page := LevelPage{
		Base: web.Base{User: auth.CurrentUser(r), Error: errMsg},
		C:    c, D: game.Compute(c), ClassLine: classLine(c),
		Abilities: game.Abilities, Names: game.AbilityNames,
	}
	for _, cl := range srd.Get().Classes {
		current := 0
		sub := ""
		if e := c.Class(cl.Index); e != nil {
			current, sub = e.Level, e.Subclass
		}
		opt := LevelOption{Class: cl, Current: current, Next: current + 1, HasASI: game.HasASI(cl.Index, current+1), Subclass: sub}
		if err := game.MulticlassError(c, cl.Index); err != nil {
			opt.Blocked = err.Error()
		}
		for _, s := range cl.Subclasses {
			if s.MinLevel() <= opt.Next {
				opt.Subclasses = append(opt.Subclasses, s)
			}
		}
		page.Options = append(page.Options, opt)
	}
	return page
}

func (h *Handler) levelForm(w http.ResponseWriter, r *http.Request) {
	id, _ := charID(r)
	c, err := h.Store.Get(id, access(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	web.Render(w, h.levelTmpl, "base", h.levelPage(r, c, ""))
}

func (h *Handler) levelUp(w http.ResponseWriter, r *http.Request) {
	id, _ := charID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	in := game.LevelUpInput{
		Class:    r.PostFormValue("class"),
		Subclass: r.PostFormValue("subclass_" + r.PostFormValue("class")),
		HPMethod: r.PostFormValue("hp_method"),
		HPManual: formInt(r, "hp_manual", 0),
		ASI:      [2]string{r.PostFormValue("asi_1"), r.PostFormValue("asi_2")},
	}
	err := h.Store.Update(id, access(r), func(m *game.Mut) error {
		gain, err := m.C.LevelUp(in)
		if err != nil {
			return failErr(err)
		}
		m.Resync = true
		return m.LogEvent("Level Up", "Now level "+strconv.Itoa(m.C.Level())+", gained "+strconv.Itoa(gain)+" on the hit die.")
	})
	var ue userError
	if errors.As(err, &ue) {
		c, gerr := h.Store.Get(id, access(r))
		if gerr != nil {
			internal(w, r, gerr)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		web.Render(w, h.levelTmpl, "base", h.levelPage(r, c, string(ue)))
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/sheets/"+strconv.FormatInt(id, 10)+"?tab=features", http.StatusSeeOther)
}

// EditPage is the form for changing anything by hand.
type EditPage struct {
	web.Base
	C          *game.Character
	Abilities  []EditAbility
	Skills     []EditSkill
	Classes    []EditClass
	Alignments []string
	Armor      string
	Weapons    string
	Tools      string
	Languages  string
	Other      string
	Sizes      []string
}

type EditAbility struct {
	Key   string
	Name  string
	Base  int
	Bonus int
	Save  bool
}

type EditSkill struct {
	Index string
	Name  string
	Level string // none, proficient or expertise
}

type EditClass struct {
	Entry      game.ClassEntry
	Name       string
	Subclasses []srd.Subclass
}

func (h *Handler) editPage(r *http.Request, c *game.Character, errMsg string) EditPage {
	page := EditPage{
		Base: web.Base{User: auth.CurrentUser(r), Error: errMsg},
		C:    c, Alignments: game.Alignments,
		Armor: profLines(c, "armor"), Weapons: profLines(c, "weapon"), Tools: profLines(c, "tool"),
		Languages: profLines(c, "language"), Other: profLines(c, "other"),
		Sizes: []string{"Tiny", "Small", "Medium", "Large", "Huge", "Gargantuan"},
	}
	for i, key := range game.Abilities {
		page.Abilities = append(page.Abilities, EditAbility{key, game.AbilityNames[key], c.Base[i], c.Bonus[i], c.HasProf("save", key)})
	}
	for _, s := range game.Skills {
		level := "none"
		for _, p := range c.Profs {
			if p.Kind == "skill" && p.Name == s.Index {
				level = "proficient"
				if p.Expertise {
					level = "expertise"
				}
			}
		}
		page.Skills = append(page.Skills, EditSkill{s.Index, s.Name, level})
	}
	for _, e := range c.Classes {
		ec := EditClass{Entry: e, Name: e.Index}
		if cl, ok := srd.Get().Class(e.Index); ok {
			ec.Name = cl.Name
			ec.Subclasses = cl.Subclasses
		}
		page.Classes = append(page.Classes, ec)
	}
	return page
}

func (h *Handler) editForm(w http.ResponseWriter, r *http.Request) {
	id, _ := charID(r)
	c, err := h.Store.Get(id, access(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	web.Render(w, h.editTmpl, "base", h.editPage(r, c, ""))
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	id, _ := charID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	err := h.Store.Update(id, access(r), func(m *game.Mut) error {
		m.Resync = true
		return applyEdit(m.C, r)
	})
	var ue userError
	if errors.As(err, &ue) {
		c, gerr := h.Store.Get(id, access(r))
		if gerr != nil {
			internal(w, r, gerr)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		web.Render(w, h.editTmpl, "base", h.editPage(r, c, string(ue)))
		return
	}
	if err != nil {
		internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/sheets/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func applyEdit(c *game.Character, r *http.Request) error {
	name := clip(r.PostFormValue("name"), 100)
	if name == "" {
		return fail("Give your character a name.")
	}
	c.Name = name
	c.RaceName = clip(r.PostFormValue("race_name"), 100)
	c.Background = clip(r.PostFormValue("background"), 100)
	c.Alignment = clip(r.PostFormValue("alignment"), 40)
	c.Size = clip(r.PostFormValue("size"), 20)
	c.Experience = clampInt(formInt(r, "experience", c.Experience), 0, 100000000)
	c.Speed = clampInt(formInt(r, "speed", c.Speed), 0, 1000)
	c.HPBase = clampInt(formInt(r, "hp_base", c.HPBase), 0, maxAmount)
	c.HPPerLevel = clampInt(formInt(r, "hp_per_level", c.HPPerLevel), -100, 100)
	c.HPBonus = clampInt(formInt(r, "hp_bonus", c.HPBonus), -maxAmount, maxAmount)
	c.ACBonus = clampInt(formInt(r, "ac_bonus", c.ACBonus), -50, 50)
	c.InitBonus = clampInt(formInt(r, "init_bonus", c.InitBonus), -50, 50)
	c.SpeedBonus = clampInt(formInt(r, "speed_bonus", c.SpeedBonus), -1000, 1000)
	c.SaveBonus = clampInt(formInt(r, "save_bonus", c.SaveBonus), -50, 50)

	for i, key := range game.Abilities {
		c.Base[i] = clampInt(formInt(r, "base_"+key, c.Base[i]), 1, 30)
		c.Bonus[i] = clampInt(formInt(r, "bonus_"+key, c.Bonus[i]), -30, 30)
	}

	var classes []game.ClassEntry
	total := 0
	for _, e := range c.Classes {
		level := clampInt(formInt(r, "level_"+e.Index, e.Level), 0, game.MaxLevel)
		if level == 0 {
			continue
		}
		e.Level = level
		e.HitDiceUsed = min(e.HitDiceUsed, level)
		sub := r.PostFormValue("subclass_" + e.Index)
		if sub != "" {
			cl, ok := srd.Get().Class(e.Index)
			if !ok {
				return fail("Unknown class.")
			}
			if _, ok := cl.Subclass(sub); !ok {
				return fail("That subclass doesn't belong to this class.")
			}
		}
		e.Subclass = sub
		total += level
		classes = append(classes, e)
	}
	if len(classes) == 0 {
		return fail("A character needs at least one class level.")
	}
	if total > game.MaxLevel {
		return fail("Characters stop at level 20.")
	}
	c.Classes = classes

	var profs []game.Prof
	for _, key := range game.Abilities {
		if r.PostFormValue("save_"+key) != "" {
			profs = append(profs, game.Prof{Kind: "save", Name: key})
		}
	}
	for _, s := range game.Skills {
		switch r.PostFormValue("skill_" + s.Index) {
		case "proficient":
			profs = append(profs, game.Prof{Kind: "skill", Name: s.Index})
		case "expertise":
			profs = append(profs, game.Prof{Kind: "skill", Name: s.Index, Expertise: true})
		}
	}
	refs := map[string]string{}
	for _, p := range c.Profs {
		refs[p.Kind+"|"+p.Name] = p.Ref
	}
	for field, kind := range map[string]string{"armor": "armor", "weapons": "weapon", "tools": "tool", "languages": "language", "other": "other"} {
		for _, name := range strings.Split(r.PostFormValue(field), "\n") {
			name = clip(name, 80)
			if name != "" {
				profs = append(profs, game.Prof{Kind: kind, Name: name, Ref: refs[kind+"|"+name]})
			}
		}
	}
	c.Profs = nil
	for _, p := range profs {
		c.AddProf(p)
	}
	c.HPCurrent = min(c.HPCurrent, game.MaxHP(c))
	return nil
}
