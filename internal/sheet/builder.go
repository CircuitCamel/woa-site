package sheet

import (
	"errors"
	"fmt"
	"strings"
	"warofages/internal/srd"
)

// CustomOption marks a free-form race or background in the builder.
const CustomOption = "custom"

// Draft is what the character builder submits. Build turns it into a Character
// for both the live preview and saving.
type Draft struct {
	Name             string
	Race             string // "race" or "race/subrace" or CustomOption
	RaceCustom       string
	Class            string
	Subclass         string
	Level            int
	Background       string // SRD index or CustomOption
	BackgroundCustom string
	Alignment        string
	Method           string // standard, point or manual
	Scores           [6]int
	Skills           []string
	BonusChoices     []string
}

// DefaultDraft is the builder's starting state.
func DefaultDraft() Draft {
	return Draft{Level: 1, Method: "standard", Scores: [6]int{15, 14, 13, 12, 10, 8}, Class: "fighter", Race: "human", Background: "acolyte", Alignment: "True Neutral"}
}

// Build validates a draft and produces the character, with its point buy total.
func (d Draft) Build() (*Character, int, error) {
	data := srd.Get()
	c := &Character{
		Name:          strings.TrimSpace(d.Name),
		Alignment:     d.Alignment,
		Speed:         30,
		Size:          "Medium",
		ResourcesUsed: map[string]int{},
		Base:          d.Scores,
	}
	spent, err := ValidateScores(d.Method, d.Scores)
	if err != nil {
		return nil, spent, err
	}
	if c.Name == "" {
		return nil, spent, errors.New("give your character a name")
	}
	if len(c.Name) > 100 {
		return nil, spent, errors.New("that name is too long")
	}
	if d.Level < 1 || d.Level > MaxLevel {
		return nil, spent, fmt.Errorf("starting level must be between 1 and %d", MaxLevel)
	}
	if err := d.applyRace(c); err != nil {
		return nil, spent, err
	}

	cl, ok := data.Class(d.Class)
	if !ok {
		return nil, spent, errors.New("choose a class")
	}
	entry := ClassEntry{Index: cl.Index, Level: d.Level}
	if d.Subclass != "" {
		sub, ok := cl.Subclass(d.Subclass)
		if !ok {
			return nil, spent, errors.New("that subclass doesn't belong to this class")
		}
		if sub.MinLevel() > d.Level {
			return nil, spent, fmt.Errorf("%s isn't available until level %d", sub.Name, sub.MinLevel())
		}
		entry.Subclass = sub.Index
	}
	c.Classes = []ClassEntry{entry}
	c.HPBase = cl.HitDie + (d.Level-1)*(cl.HitDie/2+1)
	for _, s := range cl.Saves {
		c.AddProf(Prof{Kind: "save", Name: s})
	}
	for _, p := range cl.Proficiencies {
		c.AddProf(Prof{Kind: p.Kind, Name: p.Name, Ref: p.Ref})
	}
	for _, s := range d.Skills {
		if _, ok := SkillByIndex(s); ok {
			c.AddProf(Prof{Kind: "skill", Name: s})
		}
	}
	for _, si := range cl.StartingItems {
		addStartItem(c, si)
	}

	if err := d.applyBackground(c); err != nil {
		return nil, spent, err
	}
	c.Features = append(c.Features, DesiredFeatures(c)...)
	c.Spells = append(c.Spells, DesiredSpells(c)...)
	c.HPCurrent = MaxHP(c)
	return c, spent, nil
}

func (d Draft) applyRace(c *Character) error {
	if d.Race == CustomOption {
		c.RaceName = strings.TrimSpace(d.RaceCustom)
		if c.RaceName == "" {
			return errors.New("enter a name for your custom race")
		}
		c.RaceIndex = CustomOption
		return nil
	}
	raceIndex, subIndex, _ := strings.Cut(d.Race, "/")
	race, ok := srd.Get().Race(raceIndex)
	if !ok {
		return errors.New("choose a race")
	}
	c.RaceIndex = race.Index
	c.RaceName = race.Name
	c.Speed = race.Speed
	c.Size = race.Size
	applyBonuses(c, race.Bonuses)
	for _, p := range race.Proficiencies {
		c.AddProf(Prof{Kind: p.Kind, Name: p.Name, Ref: p.Ref})
	}
	for _, l := range race.Languages {
		c.AddProf(Prof{Kind: "language", Name: l})
	}
	addTraits(c, race.Traits, race.Name)

	if race.BonusChoose > 0 {
		picked := map[string]bool{}
		for _, key := range d.BonusChoices {
			valid := false
			for _, o := range race.BonusOptions {
				valid = valid || o == key
			}
			if valid && !picked[key] && len(picked) < race.BonusChoose {
				picked[key] = true
				c.Bonus[AbilityIndex(key)]++
			}
		}
	}

	if subIndex != "" {
		sub, ok := race.Subrace(subIndex)
		if !ok {
			return errors.New("that subrace doesn't belong to this race")
		}
		c.SubraceIndex = sub.Index
		c.RaceName = sub.Name
		applyBonuses(c, sub.Bonuses)
		for _, p := range sub.Proficiencies {
			c.AddProf(Prof{Kind: p.Kind, Name: p.Name, Ref: p.Ref})
		}
		c.HPPerLevel = sub.HPPerLevel
		addTraits(c, sub.Traits, sub.Name)
	}
	return nil
}

func applyBonuses(c *Character, list []srd.AbilityBonus) {
	for _, b := range list {
		if i := AbilityIndex(b.Ability); i >= 0 {
			c.Bonus[i] += b.Bonus
		}
	}
}

func addTraits(c *Character, traits []srd.Trait, source string) {
	for _, t := range traits {
		c.Features = append(c.Features, Feature{SRDIndex: "race:" + t.Index, Name: t.Name, Source: source, Description: t.Desc})
	}
}

func (d Draft) applyBackground(c *Character) error {
	if d.Background == CustomOption {
		c.Background = strings.TrimSpace(d.BackgroundCustom)
		return nil
	}
	bg, ok := srd.Get().Background(d.Background)
	if !ok {
		return errors.New("choose a background")
	}
	c.Background = bg.Name
	c.GP += bg.Gold
	for _, s := range bg.Skills {
		c.AddProf(Prof{Kind: "skill", Name: s})
	}
	for _, si := range bg.Items {
		addStartItem(c, si)
	}
	c.Features = append(c.Features, Feature{SRDIndex: "background:" + bg.Index, Name: bg.FeatureName, Source: bg.Name + " background", Description: bg.FeatureDesc})
	return nil
}

func addStartItem(c *Character, si srd.StartItem) {
	data, ok := srd.Get().Item(si.Index)
	if !ok {
		return
	}
	for i := range c.Items {
		if c.Items[i].SRDIndex == si.Index {
			c.Items[i].Qty += si.Qty
			return
		}
	}
	it := ItemFromSRD(*data)
	it.Qty = si.Qty
	c.Items = append(c.Items, it)
}

// ItemFromSRD copies catalog data into a new inventory item.
func ItemFromSRD(s srd.Item) Item {
	return Item{
		SRDIndex: s.Index, Name: s.Name, Kind: s.Kind, Qty: 1, Weight: s.Weight, Cost: s.Cost, Description: s.Desc,
		RequiresAttunement: s.RequiresAttunement, MagicBonus: s.MagicBonus,
		ArmorCategory: s.ArmorCategory, ArmorBase: s.ArmorBase, ArmorDex: s.ArmorDex, ArmorMaxDex: s.ArmorMaxDex,
		StrMin: s.StrMin, StealthDis: s.StealthDis,
		WeaponCategory: s.WeaponCategory, WeaponRange: s.WeaponRange, DamageDice: s.DamageDice, DamageType: s.DamageType,
		TwoHandedDice: s.TwoHandedDice, Properties: strings.Join(s.Properties, ", "), RangeText: s.Range,
	}
}

// SpellFromSRD copies catalog data into a new known spell.
func SpellFromSRD(s srd.Spell, sourceClass string) Spell {
	return Spell{
		SRDIndex: s.Index, Name: s.Name, Level: s.Level, School: s.School, CastingTime: s.CastingTime,
		RangeText: s.Range, Components: s.Components, Duration: s.Duration, Description: s.Desc, HigherLevel: s.Higher,
		Ritual: s.Ritual, Concentration: s.Concentration, SourceClass: sourceClass, AttackType: s.AttackType, SaveAbility: s.SaveAbility,
		DamageType: s.DamageType,
	}
}

// DesiredSpells lists the always-prepared spells that subclasses grant at the
// character's current levels.
func DesiredSpells(c *Character) []Spell {
	data := srd.Get()
	var out []Spell
	for _, e := range c.Classes {
		cl, ok := data.Class(e.Index)
		if !ok {
			continue
		}
		sub, ok := cl.Subclass(e.Subclass)
		if !ok {
			continue
		}
		for _, grant := range sub.Spells {
			if grant.Level > e.Level {
				continue
			}
			if s, ok := data.Spell(grant.Spell); ok {
				sp := SpellFromSRD(*s, e.Index)
				sp.Prepared, sp.AlwaysPrepared = true, true
				out = append(out, sp)
			}
		}
	}
	return out
}
