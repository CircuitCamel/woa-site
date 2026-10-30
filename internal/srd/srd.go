// Package srd exposes the D&D 5e SRD 5.1 game data embedded in data/. The files
// are produced by internal/srd/gen (make srd).
package srd

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed data
var dataFS embed.FS

// Proficiency is a single armor, weapon, tool, skill, save or other proficiency.
// Ref is the equipment or category index it covers, when it has one.
type Proficiency struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Ref  string `json:"ref,omitempty"`
}

type Prereq struct {
	Ability string `json:"ability"`
	Min     int    `json:"min"`
}

type StartItem struct {
	Index string `json:"index"`
	Qty   int    `json:"qty"`
}

type ClassLevel struct {
	Level       int            `json:"level"`
	ProfBonus   int            `json:"prof_bonus"`
	Features    []string       `json:"features,omitempty"`
	Cantrips    int            `json:"cantrips,omitempty"`
	SpellsKnown int            `json:"spells_known,omitempty"`
	Slots       [9]int         `json:"slots"`
	Specific    map[string]int `json:"specific,omitempty"`
}

type SubclassLevel struct {
	Level    int      `json:"level"`
	Features []string `json:"features,omitempty"`
}

type SubclassSpell struct {
	Level int    `json:"level"`
	Spell string `json:"spell"`
}

type Subclass struct {
	Index  string          `json:"index"`
	Name   string          `json:"name"`
	Flavor string          `json:"flavor"`
	Desc   string          `json:"desc"`
	Levels []SubclassLevel `json:"levels"`
	Spells []SubclassSpell `json:"spells,omitempty"`
}

// MinLevel is the class level at which the subclass is chosen.
func (s Subclass) MinLevel() int {
	min := 0
	for _, l := range s.Levels {
		if min == 0 || l.Level < min {
			min = l.Level
		}
	}
	return min
}

type Class struct {
	Index           string        `json:"index"`
	Name            string        `json:"name"`
	HitDie          int           `json:"hit_die"`
	Saves           []string      `json:"saves"`
	SpellAbility    string        `json:"spell_ability,omitempty"`
	Caster          string        `json:"caster,omitempty"` // full, half, pact or empty
	Prepares        bool          `json:"prepares,omitempty"`
	Proficiencies   []Proficiency `json:"proficiencies"`
	SkillChoose     int           `json:"skill_choose"`
	SkillOptions    []string      `json:"skill_options"`
	OtherChoices    []string      `json:"other_choices,omitempty"`
	PrereqAll       []Prereq      `json:"prereq_all,omitempty"`
	PrereqAny       []Prereq      `json:"prereq_any,omitempty"`
	MulticlassProfs []Proficiency `json:"multiclass_profs,omitempty"`
	StartingItems   []StartItem   `json:"starting_items,omitempty"`
	StartingOptions []string      `json:"starting_options,omitempty"`
	Levels          []ClassLevel  `json:"levels"`
	Subclasses      []Subclass    `json:"subclasses"`
}

// Level returns the table row for a class level, clamped to 1-20.
func (c Class) Level(n int) ClassLevel {
	if n < 1 {
		n = 1
	}
	if n > len(c.Levels) {
		n = len(c.Levels)
	}
	return c.Levels[n-1]
}

// Subclass finds one of the class's subclasses by index.
func (c Class) Subclass(index string) (Subclass, bool) {
	for _, s := range c.Subclasses {
		if s.Index == index {
			return s, true
		}
	}
	return Subclass{}, false
}

type Trait struct {
	Index string `json:"index"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
}

type AbilityBonus struct {
	Ability string `json:"ability"`
	Bonus   int    `json:"bonus"`
}

type Subrace struct {
	Index         string         `json:"index"`
	Name          string         `json:"name"`
	Desc          string         `json:"desc"`
	Bonuses       []AbilityBonus `json:"bonuses"`
	Traits        []Trait        `json:"traits"`
	Proficiencies []Proficiency  `json:"proficiencies,omitempty"`
	HPPerLevel    int            `json:"hp_per_level,omitempty"`
}

type Race struct {
	Index          string         `json:"index"`
	Name           string         `json:"name"`
	Speed          int            `json:"speed"`
	Size           string         `json:"size"`
	Bonuses        []AbilityBonus `json:"bonuses"`
	BonusChoose    int            `json:"bonus_choose,omitempty"`
	BonusOptions   []string       `json:"bonus_options,omitempty"`
	Languages      []string       `json:"languages"`
	LanguageChoose int            `json:"language_choose,omitempty"`
	Proficiencies  []Proficiency  `json:"proficiencies,omitempty"`
	ChoiceHints    []string       `json:"choice_hints,omitempty"`
	Traits         []Trait        `json:"traits"`
	Subraces       []Subrace      `json:"subraces,omitempty"`
}

// Subrace finds one of the race's subraces by index.
func (r Race) Subrace(index string) (Subrace, bool) {
	for _, s := range r.Subraces {
		if s.Index == index {
			return s, true
		}
	}
	return Subrace{}, false
}

type Background struct {
	Index          string      `json:"index"`
	Name           string      `json:"name"`
	Skills         []string    `json:"skills"`
	LanguageChoose int         `json:"language_choose,omitempty"`
	FeatureName    string      `json:"feature_name"`
	FeatureDesc    string      `json:"feature_desc"`
	Gold           int         `json:"gold"`
	Items          []StartItem `json:"items,omitempty"`
}

type Spell struct {
	Index         string            `json:"index"`
	Name          string            `json:"name"`
	Level         int               `json:"level"`
	School        string            `json:"school"`
	CastingTime   string            `json:"casting_time"`
	Range         string            `json:"range"`
	Components    string            `json:"components"`
	Duration      string            `json:"duration"`
	Ritual        bool              `json:"ritual,omitempty"`
	Concentration bool              `json:"concentration,omitempty"`
	Desc          string            `json:"desc"`
	Higher        string            `json:"higher,omitempty"`
	Classes       []string          `json:"classes"`
	AttackType    string            `json:"attack_type,omitempty"`
	SaveAbility   string            `json:"save_ability,omitempty"`
	DamageType    string            `json:"damage_type,omitempty"`
	DamageBySlot  map[string]string `json:"damage_by_slot,omitempty"`
	DamageByLevel map[string]string `json:"damage_by_level,omitempty"`
	HealBySlot    map[string]string `json:"heal_by_slot,omitempty"`
}

type Item struct {
	Index              string   `json:"index"`
	Name               string   `json:"name"`
	Kind               string   `json:"kind"` // weapon, armor, shield, gear, tool, mount, magic
	Category           string   `json:"category"`
	Weight             float64  `json:"weight,omitempty"`
	Cost               string   `json:"cost,omitempty"`
	Desc               string   `json:"desc,omitempty"`
	Rarity             string   `json:"rarity,omitempty"`
	RequiresAttunement bool     `json:"requires_attunement,omitempty"`
	MagicBonus         int      `json:"magic_bonus,omitempty"`
	ArmorCategory      string   `json:"armor_category,omitempty"`
	ArmorBase          int      `json:"armor_base,omitempty"`
	ArmorDex           bool     `json:"armor_dex,omitempty"`
	ArmorMaxDex        int      `json:"armor_max_dex,omitempty"`
	StrMin             int      `json:"str_min,omitempty"`
	StealthDis         bool     `json:"stealth_dis,omitempty"`
	WeaponCategory     string   `json:"weapon_category,omitempty"`
	WeaponRange        string   `json:"weapon_range,omitempty"`
	DamageDice         string   `json:"damage_dice,omitempty"`
	DamageType         string   `json:"damage_type,omitempty"`
	TwoHandedDice      string   `json:"two_handed_dice,omitempty"`
	Properties         []string `json:"properties,omitempty"`
	Range              string   `json:"range,omitempty"`
}

type Condition struct {
	Index string `json:"index"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
}

type Feature struct {
	Index    string `json:"index"`
	Name     string `json:"name"`
	Class    string `json:"class"`
	Subclass string `json:"subclass,omitempty"`
	Level    int    `json:"level"`
	Desc     string `json:"desc"`
}

// Data holds every embedded dataset.
type Data struct {
	Classes     []Class
	Races       []Race
	Backgrounds []Background
	Spells      []Spell
	Items       []Item
	Conditions  []Condition
	Features    map[string]Feature

	classes map[string]*Class
	races   map[string]*Race
	spells  map[string]*Spell
	items   map[string]*Item
}

var (
	loadOnce sync.Once
	loaded   *Data
)

// Get parses the embedded data on first use. The data is fixed at build time,
// so a parse failure is a programming error.
func Get() *Data {
	loadOnce.Do(func() {
		d := &Data{}
		for name, target := range map[string]any{
			"classes":     &d.Classes,
			"races":       &d.Races,
			"backgrounds": &d.Backgrounds,
			"spells":      &d.Spells,
			"items":       &d.Items,
			"conditions":  &d.Conditions,
			"features":    &d.Features,
		} {
			raw, err := dataFS.ReadFile("data/" + name + ".json")
			if err != nil {
				panic(fmt.Errorf("srd: read %s: %w", name, err))
			}
			if err := json.Unmarshal(raw, target); err != nil {
				panic(fmt.Errorf("srd: parse %s: %w", name, err))
			}
		}
		d.classes = map[string]*Class{}
		for i := range d.Classes {
			d.classes[d.Classes[i].Index] = &d.Classes[i]
		}
		d.races = map[string]*Race{}
		for i := range d.Races {
			d.races[d.Races[i].Index] = &d.Races[i]
		}
		d.spells = map[string]*Spell{}
		for i := range d.Spells {
			d.spells[d.Spells[i].Index] = &d.Spells[i]
		}
		d.items = map[string]*Item{}
		for i := range d.Items {
			d.items[d.Items[i].Index] = &d.Items[i]
		}
		loaded = d
	})
	return loaded
}

func (d *Data) Class(index string) (*Class, bool) { c, ok := d.classes[index]; return c, ok }
func (d *Data) Race(index string) (*Race, bool)   { r, ok := d.races[index]; return r, ok }
func (d *Data) Spell(index string) (*Spell, bool) { s, ok := d.spells[index]; return s, ok }
func (d *Data) Item(index string) (*Item, bool)   { i, ok := d.items[index]; return i, ok }

// Background finds a background by index.
func (d *Data) Background(index string) (*Background, bool) {
	for i := range d.Backgrounds {
		if d.Backgrounds[i].Index == index {
			return &d.Backgrounds[i], true
		}
	}
	return nil, false
}

// Condition finds a condition by index.
func (d *Data) Condition(index string) (*Condition, bool) {
	for i := range d.Conditions {
		if d.Conditions[i].Index == index {
			return &d.Conditions[i], true
		}
	}
	return nil, false
}

// SpellQuery filters SearchSpells. Level -1 means any level.
type SpellQuery struct {
	Text  string
	Level int
	Class string
}

// SearchSpells returns matching spells by level then name, with the total count
// before limit is applied.
func (d *Data) SearchSpells(q SpellQuery, limit int) ([]Spell, int) {
	text := strings.ToLower(strings.TrimSpace(q.Text))
	var out []Spell
	for _, s := range d.Spells {
		if q.Level >= 0 && s.Level != q.Level {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(s.Name), text) {
			continue
		}
		if q.Class != "" && !contains(s.Classes, q.Class) {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].Name < out[j].Name
	})
	total := len(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total
}

// SearchItems returns matching items by name, with the total count before limit
// is applied.
func (d *Data) SearchItems(text, kind string, limit int) ([]Item, int) {
	text = strings.ToLower(strings.TrimSpace(text))
	var out []Item
	for _, it := range d.Items {
		if kind != "" && it.Kind != kind {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(it.Name), text) {
			continue
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	total := len(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
