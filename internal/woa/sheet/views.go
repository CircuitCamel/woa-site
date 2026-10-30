package sheet

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"warofages/internal/auth"
	game "warofages/internal/sheet"
	"warofages/internal/srd"
	"warofages/internal/woa/web"
)

// Page is everything the sheet template shows.
type Page struct {
	web.Base
	C             *game.Character
	D             *game.Derived
	Tab           string
	Tabs          []Tab
	ClassLine     string
	Conditions    []ConditionRow
	SpellGroups   []SpellGroup
	Items         []ItemGroup
	FeatureGroups []FeatureGroup
	ProfGroups    []ProfGroup
	Rolls         []game.Roll
	LastRoll      *game.Roll
	SpellSearch   SpellResults
	ItemSearch    ItemResults
	StartOptions  []string
	Others        bool // an admin viewing someone else's sheet
	BlankItem     game.Item
}

type ConditionRow struct {
	Index  string
	Name   string
	Desc   string
	Active bool
}

type SlotOption struct {
	Value string
	Label string
}

type SpellRow struct {
	game.Spell
	CanCast    bool
	Options    []SlotOption
	Damage     string
	DamageType string
	Healing    string
	Attack     int
	HasCaster  bool
	SaveDC     int
	SaveName   string
	Source     string
	NeedsPrep  bool
}

type SpellGroup struct {
	Level int
	Label string
	Rows  []SpellRow
}

type ItemGroup struct {
	Label string
	Rows  []game.Item
}

type FeatureGroup struct {
	Label string
	Rows  []game.Feature
}

type ProfGroup struct {
	Label string
	Names string
}

type SpellResult struct {
	srd.Spell
	Source string
}

type SpellResults struct {
	SheetID  int64
	Tab      string
	Searched bool
	Query    string
	Level    int
	Class    string
	Rows     []SpellResult
	Total    int
	Casters  []game.CasterView
	Classes  []srd.Class
}

type ItemResults struct {
	SheetID  int64
	Tab      string
	Searched bool
	Query    string
	Kind     string
	Rows     []srd.Item
	Total    int
}

func classLine(c *game.Character) string {
	var parts []string
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if !ok {
			continue
		}
		part := cl.Name + " " + strconv.Itoa(e.Level)
		if sub, ok := cl.Subclass(e.Subclass); ok {
			part += " (" + sub.Name + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " / ")
}

func (h *Handler) buildPage(r *http.Request, c *game.Character, tab, errMsg string) (*Page, error) {
	d := game.Compute(c)
	u := auth.CurrentUser(r)
	rolls, err := h.Store.Rolls(c.ID, rollsShown)
	if err != nil {
		return nil, err
	}
	p := &Page{
		Base: web.Base{User: u, Error: errMsg},
		C:    c, D: d, Tab: tab, Tabs: Tabs,
		ClassLine:     classLine(c),
		Conditions:    conditionRows(c),
		SpellGroups:   spellGroups(c, d),
		Items:         itemGroups(c),
		FeatureGroups: featureGroups(c),
		ProfGroups:    profGroups(c),
		Rolls:         rolls,
		Others:        c.UserID != u.ID,
		BlankItem:     game.Item{Qty: 1, Kind: "gear"},
	}
	if len(rolls) > 0 {
		p.LastRoll = &rolls[0]
	}
	for _, e := range c.Classes {
		if cl, ok := srd.Get().Class(e.Index); ok {
			p.StartOptions = append(p.StartOptions, cl.StartingOptions...)
			break
		}
	}
	query := r.URL.Query()
	p.SpellSearch = searchSpells(c, d, tab, query.Get("spell_q"), query.Get("spell_level"), query.Get("spell_class"))
	p.ItemSearch = searchItems(c, tab, query.Get("item_q"), query.Get("item_kind"))
	return p, nil
}

func conditionRows(c *game.Character) []ConditionRow {
	var rows []ConditionRow
	for _, cond := range srd.Get().Conditions {
		rows = append(rows, ConditionRow{Index: cond.Index, Name: cond.Name, Desc: cond.Desc, Active: c.HasCondition(cond.Index)})
	}
	return rows
}

func slotOptions(d *game.Derived, level int) []SlotOption {
	var out []SlotOption
	for _, s := range d.Slots {
		if s.Level >= level && s.Remaining > 0 {
			out = append(out, SlotOption{strconv.Itoa(s.Level), fmt.Sprintf("%s level (%d left)", web.Ordinal(s.Level), s.Remaining)})
		}
	}
	if d.Pact != nil && d.Pact.Level >= level && d.Pact.Remaining > 0 {
		out = append(out, SlotOption{"pact", fmt.Sprintf("Pact, %s level (%d left)", web.Ordinal(d.Pact.Level), d.Pact.Remaining)})
	}
	return out
}

func spellGroups(c *game.Character, d *game.Derived) []SpellGroup {
	byLevel := map[int][]SpellRow{}
	for _, sp := range c.Spells {
		row := SpellRow{Spell: sp}
		mod := 0
		if caster := casterFor(d, sp.SourceClass); caster != nil {
			mod = caster.Mod
			row.HasCaster = true
			row.Attack = caster.Attack
			row.SaveDC = caster.DC
			row.Source = caster.ClassName
			row.NeedsPrep = caster.Prepares && sp.Level > 0 && !sp.AlwaysPrepared
		}
		if sp.SaveAbility != "" {
			row.SaveName = strings.ToUpper(sp.SaveAbility)
		}
		slot := sp.Level
		row.Damage, row.DamageType = game.SpellDamage(sp, slot, d.Level, mod)
		row.Healing = game.SpellHealing(sp, slot, mod)
		row.Options = slotOptions(d, sp.Level)
		row.CanCast = sp.Level == 0 || len(row.Options) > 0
		byLevel[sp.Level] = append(byLevel[sp.Level], row)
	}
	levels := make([]int, 0, len(byLevel))
	for l := range byLevel {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	var out []SpellGroup
	for _, l := range levels {
		label := "Cantrips"
		if l > 0 {
			label = web.Ordinal(l) + " Level"
		}
		out = append(out, SpellGroup{Level: l, Label: label, Rows: byLevel[l]})
	}
	return out
}

func itemGroups(c *game.Character) []ItemGroup {
	order := []struct{ kind, label string }{
		{"weapon", "Weapons"}, {"armor", "Armor"}, {"shield", "Shields"}, {"magic", "Magic Items"},
		{"gear", "Gear"}, {"tool", "Tools"}, {"mount", "Mounts & Vehicles"},
	}
	var out []ItemGroup
	for _, o := range order {
		var rows []game.Item
		for _, it := range c.Items {
			if it.Kind == o.kind {
				rows = append(rows, it)
			}
		}
		if len(rows) > 0 {
			out = append(out, ItemGroup{o.label, rows})
		}
	}
	return out
}

func featureGroups(c *game.Character) []FeatureGroup {
	var order []string
	groups := map[string][]game.Feature{}
	for _, f := range c.Features {
		if _, ok := groups[f.Source]; !ok {
			order = append(order, f.Source)
		}
		groups[f.Source] = append(groups[f.Source], f)
	}
	var out []FeatureGroup
	for _, label := range order {
		out = append(out, FeatureGroup{label, groups[label]})
	}
	return out
}

func profGroups(c *game.Character) []ProfGroup {
	order := []struct{ kind, label string }{
		{"armor", "Armor"}, {"weapon", "Weapons"}, {"tool", "Tools"}, {"language", "Languages"}, {"other", "Other"},
	}
	var out []ProfGroup
	for _, o := range order {
		names := profNames(c, o.kind)
		if names != "" {
			out = append(out, ProfGroup{o.label, names})
		}
	}
	return out
}

func profList(c *game.Character, kind string) []string {
	var names []string
	for _, p := range c.Profs {
		if p.Kind == kind {
			names = append(names, p.Name)
		}
	}
	return names
}

func profNames(c *game.Character, kind string) string {
	return strings.Join(profList(c, kind), ", ")
}

// profLines is for the edit form. Names can contain commas ("Crossbows, light"),
// so that form uses one per line.
func profLines(c *game.Character, kind string) string {
	return strings.Join(profList(c, kind), "\n")
}

func searchSpells(c *game.Character, d *game.Derived, tab, q, level, class string) SpellResults {
	res := SpellResults{SheetID: c.ID, Tab: tab, Query: q, Level: -1, Class: class, Casters: d.Casters, Classes: srd.Get().Classes}
	if n, err := strconv.Atoi(level); err == nil && n >= 0 && n <= 9 {
		res.Level = n
	}
	res.Searched = strings.TrimSpace(q) != "" || res.Level >= 0 || class != ""
	if !res.Searched {
		return res
	}
	rows, total := srd.Get().SearchSpells(srd.SpellQuery{Text: q, Level: res.Level, Class: class}, searchLimit)
	res.Total = total
	for _, s := range rows {
		s := s
		res.Rows = append(res.Rows, SpellResult{Spell: s, Source: defaultSource(c, &s)})
	}
	return res
}

func searchItems(c *game.Character, tab, q, kind string) ItemResults {
	res := ItemResults{SheetID: c.ID, Tab: tab, Query: q, Kind: kind}
	res.Searched = strings.TrimSpace(q) != "" || kind != ""
	if !res.Searched {
		return res
	}
	res.Rows, res.Total = srd.Get().SearchItems(q, kind, searchLimit)
	return res
}

// spellSearch and itemSearch return just the results, for client JS to swap in.
func (h *Handler) spellSearch(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("sheet"), 10, 64)
	c, err := h.Store.Get(id, access(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	q := r.URL.Query()
	web.Render(w, h.sheetTmpl, "spell-results", searchSpells(c, game.Compute(c), "spells", q.Get("spell_q"), q.Get("spell_level"), q.Get("spell_class")))
}

func (h *Handler) itemSearch(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("sheet"), 10, 64)
	c, err := h.Store.Get(id, access(r))
	if err != nil {
		internal(w, r, err)
		return
	}
	q := r.URL.Query()
	web.Render(w, h.sheetTmpl, "item-results", searchItems(c, "inventory", q.Get("item_q"), q.Get("item_kind")))
}
