package sheet

import (
	"strings"
	"testing"
)

func fixedDice(t *testing.T, rolls ...int) {
	t.Helper()
	orig := Die
	i := 0
	Die = func(sides int) int {
		if i >= len(rolls) {
			t.Fatalf("unexpected extra roll of d%d", sides)
		}
		i++
		return rolls[i-1]
	}
	t.Cleanup(func() { Die = orig })
}

func newChar(classes ...ClassEntry) *Character {
	return &Character{Name: "Test", Base: [6]int{10, 10, 10, 10, 10, 10}, Speed: 30, Classes: classes, ResourcesUsed: map[string]int{}}
}

func TestModifier(t *testing.T) {
	for score, want := range map[int]int{1: -5, 3: -4, 8: -1, 9: -1, 10: 0, 11: 0, 12: 1, 15: 2, 20: 5, 30: 10} {
		if got := Modifier(score); got != want {
			t.Errorf("Modifier(%d) = %d, want %d", score, got, want)
		}
	}
}

func TestProfBonus(t *testing.T) {
	for level, want := range map[int]int{1: 2, 4: 2, 5: 3, 8: 3, 9: 4, 12: 4, 13: 5, 16: 5, 17: 6, 20: 6} {
		if got := ProfBonus(level); got != want {
			t.Errorf("ProfBonus(%d) = %d, want %d", level, got, want)
		}
	}
}

func TestSpellSlots(t *testing.T) {
	tests := []struct {
		name    string
		classes []ClassEntry
		want    [9]int
	}{
		{"wizard 5", []ClassEntry{{Index: "wizard", Level: 5}}, [9]int{4, 3, 2}},
		{"single paladin 3 uses own table", []ClassEntry{{Index: "paladin", Level: 3}}, [9]int{3}},
		{"paladin 1 has none", []ClassEntry{{Index: "paladin", Level: 1}}, [9]int{}},
		{"cleric 3 paladin 4 is caster level 5", []ClassEntry{{Index: "cleric", Level: 3}, {Index: "paladin", Level: 4}}, [9]int{4, 3, 2}},
		{"half casters round down", []ClassEntry{{Index: "paladin", Level: 3}, {Index: "ranger", Level: 3}}, [9]int{3}},
		{"low half casters have none", []ClassEntry{{Index: "paladin", Level: 1}, {Index: "ranger", Level: 1}}, [9]int{}},
		{"fighter adds nothing", []ClassEntry{{Index: "fighter", Level: 5}, {Index: "wizard", Level: 3}}, [9]int{4, 2}},
		{"warlock is separate", []ClassEntry{{Index: "warlock", Level: 5}}, [9]int{}},
		{"warlock doesn't add to caster level", []ClassEntry{{Index: "warlock", Level: 5}, {Index: "wizard", Level: 1}}, [9]int{2}},
		{"wizard 20", []ClassEntry{{Index: "wizard", Level: 20}}, [9]int{4, 3, 3, 3, 3, 2, 2, 1, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SpellSlots(newChar(tt.classes...)); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPactSlots(t *testing.T) {
	n, level := PactSlots(newChar(ClassEntry{Index: "warlock", Level: 5}))
	if n != 2 || level != 3 {
		t.Errorf("warlock 5: got %d slots of level %d", n, level)
	}
}

func TestArmorClass(t *testing.T) {
	leather := Item{Kind: "armor", Name: "Leather", Equipped: true, ArmorCategory: "Light", ArmorBase: 11, ArmorDex: true}
	scale := Item{Kind: "armor", Name: "Scale", Equipped: true, ArmorCategory: "Medium", ArmorBase: 14, ArmorDex: true, ArmorMaxDex: 2}
	plate := Item{Kind: "armor", Name: "Plate", Equipped: true, ArmorCategory: "Heavy", ArmorBase: 18, StrMin: 15}
	shield := Item{Kind: "shield", Name: "Shield", Equipped: true, ArmorCategory: "Shield", ArmorBase: 2}
	ring := Item{Kind: "magic", Name: "Ring", Equipped: true, ACBonus: 1, RequiresAttunement: true}

	tests := []struct {
		name    string
		classes []ClassEntry
		base    [6]int
		items   []Item
		want    int
	}{
		{"unarmored", nil, [6]int{10, 14, 10, 10, 10, 10}, nil, 12},
		{"light uses all dex", nil, [6]int{10, 18, 10, 10, 10, 10}, []Item{leather}, 15},
		{"medium caps dex", nil, [6]int{10, 18, 10, 10, 10, 10}, []Item{scale}, 16},
		{"heavy ignores dex", nil, [6]int{15, 18, 10, 10, 10, 10}, []Item{plate}, 18},
		{"shield adds two", nil, [6]int{10, 14, 10, 10, 10, 10}, []Item{leather, shield}, 15},
		{"barbarian unarmored", []ClassEntry{{Index: "barbarian", Level: 1}}, [6]int{10, 14, 16, 10, 10, 10}, nil, 15},
		{"barbarian keeps it with shield", []ClassEntry{{Index: "barbarian", Level: 1}}, [6]int{10, 14, 16, 10, 10, 10}, []Item{shield}, 17},
		{"monk unarmored", []ClassEntry{{Index: "monk", Level: 1}}, [6]int{10, 14, 10, 10, 16, 10}, nil, 15},
		{"monk loses it with shield", []ClassEntry{{Index: "monk", Level: 1}}, [6]int{10, 14, 10, 10, 16, 10}, []Item{shield}, 14},
		{"draconic resilience", []ClassEntry{{Index: "sorcerer", Level: 1, Subclass: "draconic"}}, [6]int{10, 14, 10, 10, 10, 10}, nil, 15},
		{"unattuned ring gives nothing", nil, [6]int{10, 10, 10, 10, 10, 10}, []Item{ring}, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChar(tt.classes...)
			c.Base, c.Items = tt.base, tt.items
			if got := Compute(c).AC; got != tt.want {
				t.Errorf("AC = %d, want %d", got, tt.want)
			}
		})
	}

	c := newChar()
	attuned := ring
	attuned.Attuned = true
	c.Items = []Item{attuned}
	if got := Compute(c).AC; got != 11 {
		t.Errorf("attuned ring AC = %d, want 11", got)
	}
}

func TestSkillsAndPassives(t *testing.T) {
	c := newChar(ClassEntry{Index: "rogue", Level: 5})
	c.Base = [6]int{10, 16, 10, 10, 14, 10}
	c.Profs = []Prof{{Kind: "skill", Name: "stealth", Expertise: true}, {Kind: "skill", Name: "perception"}}
	d := Compute(c)
	got := map[string]int{}
	for _, s := range d.Skills {
		got[s.Index] = s.Bonus
	}
	if got["stealth"] != 9 || got["perception"] != 5 || got["acrobatics"] != 3 {
		t.Errorf("skills wrong: %v", got)
	}
	if d.Passives[2].Name != "Perception" || d.Passives[2].Value != 15 {
		t.Errorf("passives wrong: %v", d.Passives)
	}
}

func TestJackOfAllTrades(t *testing.T) {
	c := newChar(ClassEntry{Index: "bard", Level: 2})
	d := Compute(c)
	if d.Skills[0].Bonus != 1 || !d.Skills[0].Half {
		t.Errorf("half proficiency missing: %+v", d.Skills[0])
	}
	if d.Initiative != 1 {
		t.Errorf("initiative = %d, want 1", d.Initiative)
	}
}

func TestAttacks(t *testing.T) {
	c := newChar(ClassEntry{Index: "rogue", Level: 5})
	c.Base = [6]int{8, 18, 10, 10, 10, 10}
	c.Profs = []Prof{{Kind: "weapon", Name: "Simple Weapons"}, {Kind: "weapon", Name: "Rapiers", Ref: "rapier"}}
	c.Items = []Item{
		{ID: 1, Name: "Rapier", SRDIndex: "rapier", Equipped: true, WeaponCategory: "Martial", WeaponRange: "Melee", DamageDice: "1d8", Properties: "Finesse"},
		{ID: 2, Name: "Greatsword", SRDIndex: "greatsword", Equipped: true, WeaponCategory: "Martial", WeaponRange: "Melee", DamageDice: "2d6", MagicBonus: 1},
		{ID: 3, Name: "Mace", SRDIndex: "mace", WeaponCategory: "Simple", WeaponRange: "Melee", DamageDice: "1d6"},
	}
	d := Compute(c)
	if len(d.Attacks) != 3 {
		t.Fatalf("attacks = %d, want 2 equipped plus unarmed", len(d.Attacks))
	}
	if a := d.Attacks[0]; a.Bonus != 7 || a.Damage != "1d8+4" {
		t.Errorf("rapier: %+v", a)
	}
	if a := d.Attacks[1]; a.Bonus != 0 || a.Damage != "2d6" || a.Proficient {
		t.Errorf("unproficient +1 greatsword with -1 str: %+v", a)
	}
	if a := d.Attacks[2]; a.Name != "Unarmed Strike" || a.Damage != "0" {
		t.Errorf("unarmed: %+v", a)
	}
}

func TestMonkMartialArts(t *testing.T) {
	c := newChar(ClassEntry{Index: "monk", Level: 5})
	c.Base = [6]int{10, 16, 10, 10, 10, 10}
	c.Items = []Item{{ID: 1, Name: "Dagger", Equipped: true, WeaponRange: "Melee", WeaponCategory: "Simple", DamageDice: "1d4", Properties: "Finesse, Monk"}}
	d := Compute(c)
	if d.Attacks[0].Damage != "1d6+3" {
		t.Errorf("monk dagger = %s", d.Attacks[0].Damage)
	}
	if d.Attacks[1].Damage != "1d6+3" {
		t.Errorf("monk unarmed = %s", d.Attacks[1].Damage)
	}
	if d.Speed != 40 {
		t.Errorf("monk 5 speed = %d, want 40", d.Speed)
	}
}

func TestCasterStats(t *testing.T) {
	c := newChar(ClassEntry{Index: "wizard", Level: 5})
	c.Base = [6]int{10, 10, 10, 18, 10, 10}
	d := Compute(c)
	v := d.Casters[0]
	if v.DC != 15 || v.Attack != 7 || v.PrepareLimit != 9 {
		t.Errorf("wizard: %+v", v)
	}
}

func TestMaxHPAndExhaustion(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 3})
	c.Base[2] = 14
	c.HPBase = 22
	c.HPPerLevel = 1
	if got := MaxHP(c); got != 31 {
		t.Errorf("MaxHP = %d, want 22 + 3*(2+1) = 31", got)
	}
	c.Exhaustion = 4
	if got := MaxHP(c); got != 15 {
		t.Errorf("exhausted MaxHP = %d, want 15", got)
	}
	c.Exhaustion = 2
	if got := Compute(c).Speed; got != 15 {
		t.Errorf("exhausted speed = %d, want 15", got)
	}
}

func TestPointBuy(t *testing.T) {
	spent, err := ValidateScores("point", [6]int{15, 15, 15, 8, 8, 8})
	if err != nil || spent != 27 {
		t.Errorf("got %d, %v", spent, err)
	}
	if _, err := ValidateScores("point", [6]int{15, 15, 15, 9, 8, 8}); err == nil {
		t.Error("over budget should fail")
	}
	if _, err := ValidateScores("point", [6]int{16, 8, 8, 8, 8, 8}); err == nil {
		t.Error("score above 15 should fail")
	}
	if _, err := ValidateScores("standard", [6]int{8, 15, 10, 14, 12, 13}); err != nil {
		t.Errorf("valid array rejected: %v", err)
	}
	if _, err := ValidateScores("standard", [6]int{15, 15, 13, 12, 10, 8}); err == nil {
		t.Error("repeated standard value should fail")
	}
	if _, err := ValidateScores("manual", [6]int{31, 8, 8, 8, 8, 8}); err == nil {
		t.Error("score above 30 should fail")
	}
}

func TestDamageAndHealing(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 1})
	c.HPBase = 10
	c.HPCurrent, c.HPTemp = 10, 3
	c.TakeDamage(5)
	if c.HPTemp != 0 || c.HPCurrent != 8 {
		t.Errorf("temp first: hp %d temp %d", c.HPCurrent, c.HPTemp)
	}
	c.TakeDamage(100)
	if c.HPCurrent != 0 || c.DeathFailures != 0 {
		t.Errorf("drop to 0: hp %d fails %d", c.HPCurrent, c.DeathFailures)
	}
	c.TakeDamage(1)
	if c.DeathFailures != 1 {
		t.Errorf("damage at 0 is a failure: %d", c.DeathFailures)
	}
	c.Heal(4)
	if c.HPCurrent != 4 || c.DeathFailures != 0 {
		t.Errorf("heal clears death saves: hp %d fails %d", c.HPCurrent, c.DeathFailures)
	}
	c.Heal(100)
	if c.HPCurrent != 10 {
		t.Errorf("heal caps at max: %d", c.HPCurrent)
	}
	c.SetTempHP(5)
	c.SetTempHP(3)
	if c.HPTemp != 5 {
		t.Errorf("temp hp doesn't stack: %d", c.HPTemp)
	}
}

func TestDeathSave(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 1})
	c.DeathSave(10)
	c.DeathSave(9)
	if c.DeathSuccesses != 1 || c.DeathFailures != 1 {
		t.Errorf("got %d/%d", c.DeathSuccesses, c.DeathFailures)
	}
	c.DeathSave(1)
	if c.DeathFailures != 3 {
		t.Errorf("natural 1 is two failures: %d", c.DeathFailures)
	}
	if !Compute(c).Dead {
		t.Error("three failures should be dead")
	}
	c.DeathSave(20)
	if c.HPCurrent != 1 || c.DeathFailures != 0 {
		t.Errorf("natural 20: hp %d fails %d", c.HPCurrent, c.DeathFailures)
	}
}

func TestRests(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 3, HitDiceUsed: 3}, ClassEntry{Index: "warlock", Level: 2, HitDiceUsed: 1})
	c.Base[2] = 14
	c.HPBase = 30
	c.HPCurrent = 5
	c.PactUsed = 2
	c.Exhaustion = 2
	c.ResourcesUsed = map[string]int{"second-wind": 1, "action-surge": 1}
	c.Features = []Feature{{Name: "A", UsesMax: 2, UsesUsed: 2, Recharge: "short"}, {Name: "B", UsesMax: 1, UsesUsed: 1, Recharge: "long"}, {Name: "C", UsesMax: 1, UsesUsed: 1}}

	c.ShortRest()
	if c.PactUsed != 0 || c.ResourcesUsed["second-wind"] != 0 || c.Features[0].UsesUsed != 0 || c.Features[1].UsesUsed != 1 {
		t.Errorf("short rest: %+v", c)
	}

	c.LongRest()
	if c.HPCurrent != MaxHP(c) {
		t.Errorf("long rest HP %d, want %d", c.HPCurrent, MaxHP(c))
	}
	if c.Exhaustion != 1 {
		t.Errorf("exhaustion %d", c.Exhaustion)
	}
	if c.Features[1].UsesUsed != 0 || c.Features[2].UsesUsed != 1 {
		t.Errorf("features: %+v", c.Features)
	}
	// Level 5 regains 2 hit dice of the 4 spent.
	if used := c.Classes[0].HitDiceUsed + c.Classes[1].HitDiceUsed; used != 2 {
		t.Errorf("hit dice used after rest = %d, want 2", used)
	}
}

func TestSpendHitDie(t *testing.T) {
	fixedDice(t, 6)
	c := newChar(ClassEntry{Index: "fighter", Level: 2})
	c.Base[2] = 14
	c.HPBase = 40
	c.HPCurrent = 1
	gain, res, err := c.SpendHitDie("fighter")
	if err != nil || gain != 8 || res.Total != 8 || c.HPCurrent != 9 || c.Classes[0].HitDiceUsed != 1 {
		t.Errorf("gain %d res %+v hp %d err %v", gain, res, c.HPCurrent, err)
	}
	c.Classes[0].HitDiceUsed = 2
	if _, _, err := c.SpendHitDie("fighter"); err != ErrNoHitDice {
		t.Errorf("expected no hit dice, got %v", err)
	}
}

func TestCast(t *testing.T) {
	c := newChar(ClassEntry{Index: "wizard", Level: 3})
	spell := Spell{Name: "Sleep", Level: 1}
	if lvl, err := c.Cast(spell, 2, false, false); err != nil || lvl != 2 || c.SlotsUsed[1] != 1 {
		t.Errorf("upcast: %d %v %v", lvl, err, c.SlotsUsed)
	}
	if _, err := c.Cast(Spell{Level: 2}, 1, false, false); err != ErrSlotTooLow {
		t.Errorf("low slot: %v", err)
	}
	c.SlotsUsed[1] = 2
	if _, err := c.Cast(spell, 2, false, false); err != ErrNoSlot {
		t.Errorf("empty slot: %v", err)
	}
	if lvl, err := c.Cast(Spell{Level: 0}, 0, false, false); err != nil || lvl != 0 {
		t.Errorf("cantrip: %d %v", lvl, err)
	}
	before := c.SlotsUsed
	if _, err := c.Cast(Spell{Level: 1, Ritual: true}, 0, false, true); err != nil || c.SlotsUsed != before {
		t.Errorf("ritual: %v %v", err, c.SlotsUsed)
	}
	w := newChar(ClassEntry{Index: "warlock", Level: 5})
	if lvl, err := w.Cast(Spell{Level: 1}, 0, true, false); err != nil || lvl != 3 || w.PactUsed != 1 {
		t.Errorf("pact: %d %v %d", lvl, err, w.PactUsed)
	}
}

func TestSpellDamageScaling(t *testing.T) {
	fireball := Spell{SRDIndex: "fireball", Level: 3}
	if got, typ := SpellDamage(fireball, 5, 9, 3); got != "10d6" || typ != "Fire" {
		t.Errorf("fireball at 5: %s %s", got, typ)
	}
	firebolt := Spell{SRDIndex: "fire-bolt", Level: 0}
	if got, _ := SpellDamage(firebolt, 0, 11, 3); got != "3d10" {
		t.Errorf("fire bolt at 11: %s", got)
	}
	if got := SpellHealing(Spell{SRDIndex: "cure-wounds"}, 2, 3); got != "2d8+3" {
		t.Errorf("cure wounds at 2: %s", got)
	}
}

func TestMulticlassPrereqs(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 1})
	c.Base = [6]int{13, 10, 10, 10, 10, 10}
	if err := MulticlassError(c, "wizard"); err == nil || !strings.Contains(err.Error(), "Intelligence 13") {
		t.Errorf("wizard needs int: %v", err)
	}
	c.Base[3] = 13
	if err := MulticlassError(c, "wizard"); err != nil {
		t.Errorf("should qualify: %v", err)
	}
	c.Base[0] = 10
	if err := MulticlassError(c, "wizard"); err == nil {
		t.Error("existing fighter needs str or dex 13")
	}
	c.Base[1] = 13
	if err := MulticlassError(c, "wizard"); err != nil {
		t.Errorf("dex satisfies fighter: %v", err)
	}
	if err := MulticlassError(c, "fighter"); err != nil {
		t.Errorf("own class never blocked: %v", err)
	}
}

func TestLevelUp(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 3})
	c.Base = [6]int{19, 10, 14, 13, 10, 10}
	c.HPBase = 22
	c.HPCurrent = 10
	gain, err := c.LevelUp(LevelUpInput{Class: "fighter", HPMethod: "average", ASI: [2]string{"str", "str"}})
	if err != nil || gain != 6 {
		t.Fatalf("gain %d err %v", gain, err)
	}
	if c.Classes[0].Level != 4 || c.Score(0) != 20 {
		t.Errorf("level %d str %d (cap 20)", c.Classes[0].Level, c.Score(0))
	}
	// +6 die, +2 con, keeps damage taken.
	if c.HPCurrent != 18 {
		t.Errorf("current HP %d, want 18", c.HPCurrent)
	}

	if _, err := c.LevelUp(LevelUpInput{Class: "wizard", HPMethod: "average"}); err != nil {
		t.Fatalf("multiclass: %v", err)
	}
	if c.Class("wizard") == nil || c.Level() != 5 {
		t.Errorf("wizard not added: %+v", c.Classes)
	}
	if _, err := c.LevelUp(LevelUpInput{Class: "barbarian", HPMethod: "average"}); err != nil {
		t.Errorf("str 20 qualifies for barbarian: %v", err)
	}
	if _, err := c.LevelUp(LevelUpInput{Class: "monk", HPMethod: "average"}); err == nil {
		t.Error("monk prereqs not met")
	}
	if c.Class("monk") != nil {
		t.Error("failed level up must not add the class")
	}

	max := newChar(ClassEntry{Index: "fighter", Level: 20})
	if _, err := max.LevelUp(LevelUpInput{Class: "fighter"}); err != ErrMaxLevel {
		t.Errorf("level cap: %v", err)
	}
}

func TestLevelUpSubclassGating(t *testing.T) {
	c := newChar(ClassEntry{Index: "fighter", Level: 1})
	if _, err := c.LevelUp(LevelUpInput{Class: "fighter", Subclass: "champion"}); err == nil {
		t.Error("champion isn't available at fighter 2")
	}
	c = newChar(ClassEntry{Index: "fighter", Level: 2})
	if _, err := c.LevelUp(LevelUpInput{Class: "fighter", Subclass: "champion"}); err != nil || c.Classes[0].Subclass != "champion" {
		t.Errorf("champion at 3: %v", err)
	}
	found := false
	for _, f := range DesiredFeatures(c) {
		if f.SRDIndex == "improved-critical" {
			found = true
		}
	}
	if !found {
		t.Error("subclass feature not granted")
	}
}

func TestResources(t *testing.T) {
	c := newChar(ClassEntry{Index: "barbarian", Level: 3})
	r := computeResources(c)
	if len(r) != 1 || r[0].Key != "rage" || r[0].Max != 3 {
		t.Errorf("barbarian 3: %+v", r)
	}
	if got := computeResources(newChar(ClassEntry{Index: "barbarian", Level: 20})); len(got) != 0 {
		t.Errorf("rage is unlimited at 20: %+v", got)
	}
	b := newChar(ClassEntry{Index: "bard", Level: 5})
	b.Base[5] = 16
	r = computeResources(b)
	if r[0].Max != 3 || r[0].Recharge != "short" || !strings.Contains(r[0].Name, "d8") {
		t.Errorf("bard 5: %+v", r[0])
	}
}

func TestToggleCondition(t *testing.T) {
	c := newChar()
	if !c.ToggleCondition("poisoned") || !c.HasCondition("poisoned") {
		t.Error("should turn on")
	}
	c.ToggleCondition("poisoned")
	if c.HasCondition("poisoned") {
		t.Error("should turn off")
	}
	if c.ToggleCondition("made-up") {
		t.Error("unknown conditions are rejected")
	}
}

func TestParseDice(t *testing.T) {
	good := map[string]int{"1d6": 1, "2d6+3": 2, "d20": 1, "1d8 + 2d4 - 1": 3, "5": 1, "-1": 1}
	for expr, terms := range good {
		got, err := parseDice(expr)
		if err != nil || len(got) != terms {
			t.Errorf("parse %q: %v %v", expr, got, err)
		}
	}
	for _, expr := range []string{"", "abc", "2d", "d", "1d6+", "1d6++2", "101d6", "1d1001", "0d6", "1d0", "1d6;drop", strings.Repeat("1d4+", 30) + "1", "51d6+50d6"} {
		if _, err := parseDice(expr); err == nil {
			t.Errorf("parse %q should fail", expr)
		}
	}
}

func TestRollDice(t *testing.T) {
	fixedDice(t, 2, 5, 3)
	r, err := RollDice("2d6+1d4-1", false)
	if err != nil || r.Total != 9 || r.Detail != "[2, 5] + [3] - 1" {
		t.Errorf("%+v %v", r, err)
	}
	fixedDice(t, 1, 2, 3, 4)
	r, _ = RollDice("2d6+1", true)
	if r.Total != 11 {
		t.Errorf("critical doubles dice but not the modifier: %+v", r)
	}
}

func TestRollRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if n := Die(6); n < 1 || n > 6 {
			t.Fatalf("Die(6) = %d", n)
		}
	}
}

func TestRollD20Modes(t *testing.T) {
	fixedDice(t, 5, 17)
	if r := RollD20(2, "adv"); r.Total != 19 {
		t.Errorf("adv: %+v", r)
	}
	fixedDice(t, 5, 17)
	if r := RollD20(2, "dis"); r.Total != 7 {
		t.Errorf("dis: %+v", r)
	}
	fixedDice(t, 20)
	if r := RollD20(0, ""); r.Total != 20 || !strings.Contains(r.Detail, "natural 20") {
		t.Errorf("normal: %+v", r)
	}
}

func TestRoll4d6(t *testing.T) {
	fixedDice(t, 1, 2, 3, 4, 6, 6, 6, 1, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4)
	if got := Roll4d6(); got != [6]int{9, 18, 3, 6, 9, 12} {
		t.Errorf("got %v", got)
	}
}
