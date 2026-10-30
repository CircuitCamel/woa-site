// Command gen rebuilds internal/srd/data from the open 5e-bits/5e-database
// project (MIT licensed, SRD 5.1 content), pinned to a single commit so that
// regenerating is repeatable. Run it from the repository root with make srd.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"warofages/internal/srd"
)

const (
	commit  = "a6212beb4b278917c2cff41c2b04c4ef4f3e0d6f"
	baseURL = "https://raw.githubusercontent.com/5e-bits/5e-database/" + commit + "/src/2014/en/5e-SRD-"
	outDir  = "internal/srd/data"
)

type ref struct {
	Index string `json:"index"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type option struct {
	OptionType   string  `json:"option_type"`
	Item         ref     `json:"item"`
	Choice       *choice `json:"choice"`
	AbilityScore ref     `json:"ability_score"`
	MinimumScore int     `json:"minimum_score"`
	Bonus        int     `json:"bonus"`
}

type choice struct {
	Desc   string `json:"desc"`
	Choose int    `json:"choose"`
	From   struct {
		Options []option `json:"options"`
	} `json:"from"`
}

type abilityBonus struct {
	AbilityScore ref `json:"ability_score"`
	Bonus        int `json:"bonus"`
}

func fetch(name string, target any) {
	resp, err := http.Get(baseURL + name + ".json")
	if err != nil {
		log.Fatalf("download %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("download %s: %s", name, resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("download %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
}

func write(name string, v any) {
	raw, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		log.Fatalf("encode %s: %v", name, err)
	}
	path := filepath.Join(outDir, name+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		log.Fatalf("write %s: %v", name, err)
	}
	fmt.Printf("%s: %d bytes\n", path, len(raw))
}

func join(parts []string) string { return strings.TrimSpace(strings.Join(parts, "\n\n")) }

// profTable maps a 5e-database proficiency index to its trimmed form.
func profTable() map[string]srd.Proficiency {
	var raw []struct {
		Index     string `json:"index"`
		Type      string `json:"type"`
		Name      string `json:"name"`
		Reference ref    `json:"reference"`
	}
	fetch("Proficiencies", &raw)
	out := map[string]srd.Proficiency{}
	for _, p := range raw {
		kind := "other"
		name := p.Name
		switch p.Type {
		case "Armor":
			kind = "armor"
		case "Weapons":
			kind = "weapon"
		case "Artisan's Tools", "Gaming Sets", "Musical Instruments", "Vehicles":
			kind = "tool"
		case "Skills":
			kind = "skill"
			name = p.Reference.Index
		case "Saving Throws":
			kind = "save"
			name = strings.TrimPrefix(p.Index, "saving-throw-")
		}
		out[p.Index] = srd.Proficiency{Kind: kind, Name: name, Ref: p.Reference.Index}
	}
	return out
}

func mapProfs(table map[string]srd.Proficiency, refs []ref) []srd.Proficiency {
	var out []srd.Proficiency
	for _, r := range refs {
		p, ok := table[r.Index]
		if !ok {
			log.Fatalf("unknown proficiency %s", r.Index)
		}
		if p.Kind == "save" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func bonuses(raw []abilityBonus) []srd.AbilityBonus {
	out := []srd.AbilityBonus{}
	for _, b := range raw {
		out = append(out, srd.AbilityBonus{Ability: b.AbilityScore.Index, Bonus: b.Bonus})
	}
	return out
}

func genClasses(table map[string]srd.Proficiency) {
	var raw []struct {
		Index             string   `json:"index"`
		Name              string   `json:"name"`
		HitDie            int      `json:"hit_die"`
		ProficiencyChoice []choice `json:"proficiency_choices"`
		Proficiencies     []ref    `json:"proficiencies"`
		SavingThrows      []ref    `json:"saving_throws"`
		StartingEquipment []struct {
			Equipment ref `json:"equipment"`
			Quantity  int `json:"quantity"`
		} `json:"starting_equipment"`
		StartingOptions []struct {
			Desc string `json:"desc"`
		} `json:"starting_equipment_options"`
		MultiClassing struct {
			Prerequisites []struct {
				AbilityScore ref `json:"ability_score"`
				MinimumScore int `json:"minimum_score"`
			} `json:"prerequisites"`
			PrerequisiteOptions *choice `json:"prerequisite_options"`
			Proficiencies       []ref   `json:"proficiencies"`
		} `json:"multi_classing"`
		Spellcasting *struct {
			SpellcastingAbility ref `json:"spellcasting_ability"`
		} `json:"spellcasting"`
	}
	fetch("Classes", &raw)

	var rawLevels []struct {
		Level         int                        `json:"level"`
		ProfBonus     int                        `json:"prof_bonus"`
		Features      []ref                      `json:"features"`
		Class         ref                        `json:"class"`
		Subclass      *ref                       `json:"subclass"`
		Spellcasting  map[string]int             `json:"spellcasting"`
		ClassSpecific map[string]json.RawMessage `json:"class_specific"`
	}
	fetch("Levels", &rawLevels)

	var rawSubs []struct {
		Index  string   `json:"index"`
		Name   string   `json:"name"`
		Class  ref      `json:"class"`
		Flavor string   `json:"subclass_flavor"`
		Desc   []string `json:"desc"`
		Spells []struct {
			Prerequisites []struct {
				Index string `json:"index"`
				Type  string `json:"type"`
			} `json:"prerequisites"`
			Spell ref `json:"spell"`
		} `json:"spells"`
	}
	fetch("Subclasses", &rawSubs)

	casters := map[string]string{
		"bard": "full", "cleric": "full", "druid": "full", "sorcerer": "full", "wizard": "full",
		"paladin": "half", "ranger": "half", "warlock": "pact",
	}
	prepares := map[string]bool{"cleric": true, "druid": true, "paladin": true, "wizard": true}

	var out []srd.Class
	for _, c := range raw {
		cl := srd.Class{
			Index:           c.Index,
			Name:            c.Name,
			HitDie:          c.HitDie,
			Caster:          casters[c.Index],
			Prepares:        prepares[c.Index],
			Proficiencies:   mapProfs(table, c.Proficiencies),
			MulticlassProfs: mapProfs(table, c.MultiClassing.Proficiencies),
		}
		for _, s := range c.SavingThrows {
			cl.Saves = append(cl.Saves, s.Index)
		}
		if c.Spellcasting != nil {
			cl.SpellAbility = c.Spellcasting.SpellcastingAbility.Index
		}
		for _, ch := range c.ProficiencyChoice {
			if isSkillChoice(ch) && cl.SkillChoose == 0 {
				cl.SkillChoose = ch.Choose
				for _, o := range ch.From.Options {
					cl.SkillOptions = append(cl.SkillOptions, table[o.Item.Index].Name)
				}
			} else {
				cl.OtherChoices = append(cl.OtherChoices, ch.Desc)
			}
		}
		for _, p := range c.MultiClassing.Prerequisites {
			cl.PrereqAll = append(cl.PrereqAll, srd.Prereq{Ability: p.AbilityScore.Index, Min: p.MinimumScore})
		}
		if c.MultiClassing.PrerequisiteOptions != nil {
			for _, o := range c.MultiClassing.PrerequisiteOptions.From.Options {
				cl.PrereqAny = append(cl.PrereqAny, srd.Prereq{Ability: o.AbilityScore.Index, Min: o.MinimumScore})
			}
		}
		for _, e := range c.StartingEquipment {
			cl.StartingItems = append(cl.StartingItems, srd.StartItem{Index: e.Equipment.Index, Qty: e.Quantity})
		}
		for _, o := range c.StartingOptions {
			cl.StartingOptions = append(cl.StartingOptions, o.Desc)
		}

		for _, l := range rawLevels {
			if l.Class.Index != c.Index || l.Subclass != nil {
				continue
			}
			lvl := srd.ClassLevel{Level: l.Level, ProfBonus: l.ProfBonus, Specific: flattenSpecific(l.ClassSpecific)}
			for _, f := range l.Features {
				lvl.Features = append(lvl.Features, f.Index)
			}
			lvl.Cantrips = l.Spellcasting["cantrips_known"]
			lvl.SpellsKnown = l.Spellcasting["spells_known"]
			for i := range lvl.Slots {
				lvl.Slots[i] = l.Spellcasting[fmt.Sprintf("spell_slots_level_%d", i+1)]
			}
			cl.Levels = append(cl.Levels, lvl)
		}
		sort.Slice(cl.Levels, func(i, j int) bool { return cl.Levels[i].Level < cl.Levels[j].Level })
		if len(cl.Levels) != 20 {
			log.Fatalf("class %s has %d levels", c.Index, len(cl.Levels))
		}

		for _, s := range rawSubs {
			if s.Class.Index != c.Index {
				continue
			}
			sub := srd.Subclass{Index: s.Index, Name: s.Name, Flavor: s.Flavor, Desc: join(s.Desc)}
			for _, l := range rawLevels {
				if l.Subclass == nil || l.Subclass.Index != s.Index {
					continue
				}
				sl := srd.SubclassLevel{Level: l.Level}
				for _, f := range l.Features {
					sl.Features = append(sl.Features, f.Index)
				}
				sub.Levels = append(sub.Levels, sl)
			}
			sort.Slice(sub.Levels, func(i, j int) bool { return sub.Levels[i].Level < sub.Levels[j].Level })
			for _, sp := range s.Spells {
				level, ok := spellGrantLevel(sp.Prerequisites)
				if ok {
					sub.Spells = append(sub.Spells, srd.SubclassSpell{Level: level, Spell: sp.Spell.Index})
				}
			}
			cl.Subclasses = append(cl.Subclasses, sub)
		}
		out = append(out, cl)
	}
	write("classes", out)
}

// spellGrantLevel returns the class level a subclass spell is granted at. Spells
// that also depend on a feature choice (Circle of the Land terrain) are skipped.
func spellGrantLevel(prereqs []struct {
	Index string `json:"index"`
	Type  string `json:"type"`
}) (int, bool) {
	level := 0
	for _, p := range prereqs {
		if p.Type != "level" {
			return 0, false
		}
		n, err := strconv.Atoi(p.Index[strings.LastIndex(p.Index, "-")+1:])
		if err != nil {
			return 0, false
		}
		level = n
	}
	return level, level > 0
}

func isSkillChoice(c choice) bool {
	if len(c.From.Options) == 0 {
		return false
	}
	return strings.HasPrefix(c.From.Options[0].Item.Index, "skill-")
}

// flattenSpecific keeps the numeric class table columns. Dice columns such as
// martial_arts become martial_arts_count and martial_arts_die.
func flattenSpecific(raw map[string]json.RawMessage) map[string]int {
	out := map[string]int{}
	for k, v := range raw {
		var n int
		if json.Unmarshal(v, &n) == nil {
			if n != 0 {
				out[k] = n
			}
			continue
		}
		var dice struct {
			Count int `json:"dice_count"`
			Value int `json:"dice_value"`
		}
		if json.Unmarshal(v, &dice) == nil && dice.Value != 0 {
			out[k+"_count"] = dice.Count
			out[k+"_die"] = dice.Value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func genRaces(table map[string]srd.Proficiency) {
	var rawTraits []struct {
		Index         string   `json:"index"`
		Name          string   `json:"name"`
		Desc          []string `json:"desc"`
		Proficiencies []ref    `json:"proficiencies"`
	}
	fetch("Traits", &rawTraits)
	type traitInfo struct {
		trait srd.Trait
		profs []srd.Proficiency
	}
	traits := map[string]traitInfo{}
	for _, t := range rawTraits {
		traits[t.Index] = traitInfo{
			trait: srd.Trait{Index: t.Index, Name: t.Name, Desc: join(t.Desc)},
			profs: mapProfs(table, t.Proficiencies),
		}
	}
	collect := func(refs []ref) ([]srd.Trait, []srd.Proficiency) {
		out := []srd.Trait{}
		var profs []srd.Proficiency
		for _, r := range refs {
			t, ok := traits[r.Index]
			if !ok {
				log.Fatalf("unknown trait %s", r.Index)
			}
			out = append(out, t.trait)
			profs = append(profs, t.profs...)
		}
		return out, profs
	}

	var rawSubs []struct {
		Index          string         `json:"index"`
		Name           string         `json:"name"`
		Race           ref            `json:"race"`
		Desc           string         `json:"desc"`
		AbilityBonuses []abilityBonus `json:"ability_bonuses"`
		Starting       []ref          `json:"starting_proficiencies"`
		RacialTraits   []ref          `json:"racial_traits"`
	}
	fetch("Subraces", &rawSubs)

	var raw []struct {
		Index           string         `json:"index"`
		Name            string         `json:"name"`
		Speed           int            `json:"speed"`
		Size            string         `json:"size"`
		AbilityBonuses  []abilityBonus `json:"ability_bonuses"`
		BonusOptions    *choice        `json:"ability_bonus_options"`
		Languages       []ref          `json:"languages"`
		LanguageOptions *choice        `json:"language_options"`
		Starting        []ref          `json:"starting_proficiencies"`
		StartingOptions *choice        `json:"starting_proficiency_options"`
		Traits          []ref          `json:"traits"`
	}
	fetch("Races", &raw)

	var out []srd.Race
	for _, r := range raw {
		race := srd.Race{
			Index: r.Index, Name: r.Name, Speed: r.Speed, Size: r.Size,
			Bonuses:       bonuses(r.AbilityBonuses),
			Proficiencies: mapProfs(table, r.Starting),
			Languages:     []string{},
		}
		if r.BonusOptions != nil {
			race.BonusChoose = r.BonusOptions.Choose
			for _, o := range r.BonusOptions.From.Options {
				race.BonusOptions = append(race.BonusOptions, o.AbilityScore.Index)
			}
		}
		for _, l := range r.Languages {
			race.Languages = append(race.Languages, l.Name)
		}
		if r.LanguageOptions != nil {
			race.LanguageChoose = r.LanguageOptions.Choose
		}
		if r.StartingOptions != nil && r.StartingOptions.Desc != "" {
			race.ChoiceHints = append(race.ChoiceHints, r.StartingOptions.Desc)
		}
		var traitProfs []srd.Proficiency
		race.Traits, traitProfs = collect(r.Traits)
		race.Proficiencies = append(race.Proficiencies, traitProfs...)
		for _, s := range rawSubs {
			if s.Race.Index != r.Index {
				continue
			}
			sub := srd.Subrace{Index: s.Index, Name: s.Name, Desc: s.Desc, Bonuses: bonuses(s.AbilityBonuses), Proficiencies: mapProfs(table, s.Starting)}
			var subProfs []srd.Proficiency
			sub.Traits, subProfs = collect(s.RacialTraits)
			sub.Proficiencies = append(sub.Proficiencies, subProfs...)
			for _, t := range s.RacialTraits {
				if t.Index == "dwarven-toughness" {
					sub.HPPerLevel = 1
				}
			}
			race.Subraces = append(race.Subraces, sub)
		}
		out = append(out, race)
	}
	write("races", out)
}

func genBackgrounds(table map[string]srd.Proficiency) {
	var raw []struct {
		Index    string `json:"index"`
		Name     string `json:"name"`
		Starting []ref  `json:"starting_proficiencies"`
		Language *struct {
			Choose int `json:"choose"`
		} `json:"language_options"`
		StartingEquipment []struct {
			Equipment ref `json:"equipment"`
			Quantity  int `json:"quantity"`
		} `json:"starting_equipment"`
		StartingGold struct {
			Quantity int `json:"quantity"`
		} `json:"starting_gold"`
		Feature struct {
			Name string   `json:"name"`
			Desc []string `json:"desc"`
		} `json:"feature"`
	}
	fetch("Backgrounds", &raw)
	var out []srd.Background
	for _, b := range raw {
		bg := srd.Background{Index: b.Index, Name: b.Name, Skills: []string{}, FeatureName: b.Feature.Name, FeatureDesc: join(b.Feature.Desc), Gold: b.StartingGold.Quantity}
		for _, p := range mapProfs(table, b.Starting) {
			if p.Kind == "skill" {
				bg.Skills = append(bg.Skills, p.Name)
			}
		}
		if b.Language != nil {
			bg.LanguageChoose = b.Language.Choose
		}
		for _, e := range b.StartingEquipment {
			bg.Items = append(bg.Items, srd.StartItem{Index: e.Equipment.Index, Qty: e.Quantity})
		}
		out = append(out, bg)
	}
	write("backgrounds", out)
}

func genSpells() {
	var raw []struct {
		Index         string            `json:"index"`
		Name          string            `json:"name"`
		Desc          []string          `json:"desc"`
		Higher        []string          `json:"higher_level"`
		Range         string            `json:"range"`
		Components    []string          `json:"components"`
		Material      string            `json:"material"`
		Ritual        bool              `json:"ritual"`
		Duration      string            `json:"duration"`
		Concentration bool              `json:"concentration"`
		CastingTime   string            `json:"casting_time"`
		Level         int               `json:"level"`
		AttackType    string            `json:"attack_type"`
		School        ref               `json:"school"`
		Classes       []ref             `json:"classes"`
		Heal          map[string]string `json:"heal_at_slot_level"`
		DamageRaw     json.RawMessage   `json:"damage"`
		DC            *struct {
			Type ref `json:"dc_type"`
		} `json:"dc"`
	}
	fetch("Spells", &raw)
	var out []srd.Spell
	for _, s := range raw {
		components := strings.Join(s.Components, ", ")
		if s.Material != "" {
			components += " (" + strings.TrimSuffix(s.Material, ".") + ")"
		}
		sp := srd.Spell{
			Index: s.Index, Name: s.Name, Level: s.Level, School: s.School.Name,
			CastingTime: s.CastingTime, Range: s.Range, Components: components, Duration: s.Duration,
			Ritual: s.Ritual, Concentration: s.Concentration, Desc: join(s.Desc), Higher: join(s.Higher),
			AttackType: s.AttackType, Classes: []string{}, HealBySlot: s.Heal,
		}
		for _, c := range s.Classes {
			sp.Classes = append(sp.Classes, c.Index)
		}
		if s.DC != nil {
			sp.SaveAbility = s.DC.Type.Index
		}
		if len(s.DamageRaw) > 0 {
			var dmg struct {
				Type    ref               `json:"damage_type"`
				Slot    map[string]string `json:"damage_at_slot_level"`
				CharLvl map[string]string `json:"damage_at_character_level"`
			}
			if err := json.Unmarshal(s.DamageRaw, &dmg); err != nil {
				var list []struct {
					Type    ref               `json:"damage_type"`
					Slot    map[string]string `json:"damage_at_slot_level"`
					CharLvl map[string]string `json:"damage_at_character_level"`
				}
				if err := json.Unmarshal(s.DamageRaw, &list); err != nil {
					log.Fatalf("spell %s damage: %v", s.Index, err)
				}
				if len(list) > 0 {
					dmg = list[0]
				}
			}
			sp.DamageType = dmg.Type.Name
			sp.DamageBySlot = dmg.Slot
			sp.DamageByLevel = dmg.CharLvl
		}
		out = append(out, sp)
	}
	write("spells", out)
}

var magicBonus = regexp.MustCompile(`, \+(\d)$`)

func genItems() {
	var raw []struct {
		Index          string   `json:"index"`
		Name           string   `json:"name"`
		Desc           []string `json:"desc"`
		Category       ref      `json:"equipment_category"`
		GearCategory   *ref     `json:"gear_category"`
		ToolCategory   string   `json:"tool_category"`
		VehicleCat     string   `json:"vehicle_category"`
		ArmorCategory  string   `json:"armor_category"`
		WeaponCategory string   `json:"weapon_category"`
		WeaponRange    string   `json:"weapon_range"`
		StrMinimum     int      `json:"str_minimum"`
		StealthDis     bool     `json:"stealth_disadvantage"`
		Weight         float64  `json:"weight"`
		Cost           *struct {
			Quantity int    `json:"quantity"`
			Unit     string `json:"unit"`
		} `json:"cost"`
		ArmorClass *struct {
			Base     int  `json:"base"`
			DexBonus bool `json:"dex_bonus"`
			MaxBonus int  `json:"max_bonus"`
		} `json:"armor_class"`
		Damage *struct {
			Dice string `json:"damage_dice"`
			Type ref    `json:"damage_type"`
		} `json:"damage"`
		TwoHanded *struct {
			Dice string `json:"damage_dice"`
		} `json:"two_handed_damage"`
		Range *struct {
			Normal int `json:"normal"`
			Long   int `json:"long"`
		} `json:"range"`
		ThrowRange *struct {
			Normal int `json:"normal"`
			Long   int `json:"long"`
		} `json:"throw_range"`
		Properties []ref `json:"properties"`
	}
	fetch("Equipment", &raw)
	var out []srd.Item
	for _, e := range raw {
		it := srd.Item{Index: e.Index, Name: e.Name, Weight: e.Weight, Desc: join(e.Desc)}
		if e.Cost != nil {
			it.Cost = fmt.Sprintf("%d %s", e.Cost.Quantity, e.Cost.Unit)
		}
		switch e.Category.Index {
		case "weapon":
			it.Kind = "weapon"
			it.Category = e.WeaponCategory + " " + e.WeaponRange
			it.WeaponCategory = e.WeaponCategory
			it.WeaponRange = e.WeaponRange
			if e.Damage != nil {
				it.DamageDice = e.Damage.Dice
				it.DamageType = e.Damage.Type.Name
			}
			if e.TwoHanded != nil {
				it.TwoHandedDice = e.TwoHanded.Dice
			}
			for _, p := range e.Properties {
				it.Properties = append(it.Properties, p.Name)
			}
			switch {
			case e.ThrowRange != nil:
				it.Range = fmt.Sprintf("%d/%d ft", e.ThrowRange.Normal, e.ThrowRange.Long)
			case e.Range != nil && e.Range.Long > 0:
				it.Range = fmt.Sprintf("%d/%d ft", e.Range.Normal, e.Range.Long)
			case e.Range != nil:
				it.Range = fmt.Sprintf("%d ft", e.Range.Normal)
			}
		case "armor":
			it.Kind = "armor"
			if e.ArmorCategory == "Shield" {
				it.Kind = "shield"
			}
			it.Category = e.ArmorCategory
			it.ArmorCategory = e.ArmorCategory
			if e.ArmorClass != nil {
				it.ArmorBase = e.ArmorClass.Base
				it.ArmorDex = e.ArmorClass.DexBonus
				it.ArmorMaxDex = e.ArmorClass.MaxBonus
			}
			it.StrMin = e.StrMinimum
			it.StealthDis = e.StealthDis
		case "tools":
			it.Kind = "tool"
			it.Category = e.ToolCategory
		case "mounts-and-vehicles":
			it.Kind = "mount"
			it.Category = e.VehicleCat
		default:
			it.Kind = "gear"
			it.Category = "Adventuring Gear"
			if e.GearCategory != nil {
				it.Category = e.GearCategory.Name
			}
		}
		out = append(out, it)
	}

	var rawMagic []struct {
		Index    string   `json:"index"`
		Name     string   `json:"name"`
		Desc     []string `json:"desc"`
		Category ref      `json:"equipment_category"`
		Rarity   struct {
			Name string `json:"name"`
		} `json:"rarity"`
	}
	fetch("Magic-Items", &rawMagic)
	for _, m := range rawMagic {
		it := srd.Item{Index: "magic-" + m.Index, Name: m.Name, Kind: "magic", Category: m.Category.Name, Rarity: m.Rarity.Name, Desc: join(m.Desc)}
		it.RequiresAttunement = strings.Contains(strings.ToLower(it.Desc), "requires attunement")
		if g := magicBonus.FindStringSubmatch(m.Name); g != nil {
			it.MagicBonus, _ = strconv.Atoi(g[1])
		}
		out = append(out, it)
	}
	write("items", out)
}

func genConditions() {
	var raw []struct {
		Index string   `json:"index"`
		Name  string   `json:"name"`
		Desc  []string `json:"desc"`
	}
	fetch("Conditions", &raw)
	var out []srd.Condition
	for _, c := range raw {
		if c.Index == "exhaustion" {
			continue // tracked separately as a level
		}
		out = append(out, srd.Condition{Index: c.Index, Name: c.Name, Desc: strings.Join(c.Desc, "\n")})
	}
	write("conditions", out)
}

func genFeatures() {
	var raw []struct {
		Index    string   `json:"index"`
		Name     string   `json:"name"`
		Level    int      `json:"level"`
		Class    ref      `json:"class"`
		Subclass *ref     `json:"subclass"`
		Desc     []string `json:"desc"`
		Parent   *ref     `json:"parent"`
	}
	fetch("Features", &raw)
	out := map[string]srd.Feature{}
	for _, f := range raw {
		item := srd.Feature{Index: f.Index, Name: f.Name, Class: f.Class.Index, Level: f.Level, Desc: join(f.Desc)}
		if f.Subclass != nil {
			item.Subclass = f.Subclass.Index
		}
		out[f.Index] = item
	}
	write("features", out)
}

func main() {
	table := profTable()
	genClasses(table)
	genRaces(table)
	genBackgrounds(table)
	genSpells()
	genItems()
	genConditions()
	genFeatures()
}
