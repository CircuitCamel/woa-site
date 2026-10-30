// Package sheet holds character data, the 5e rules that derive values from it,
// and its storage. Handlers and client code never calculate game values.
package sheet

import "strings"

// Abilities in sheet order. Character.Base and Character.Bonus use these indexes.
var Abilities = []string{"str", "dex", "con", "int", "wis", "cha"}

var AbilityNames = map[string]string{
	"str": "Strength", "dex": "Dexterity", "con": "Constitution",
	"int": "Intelligence", "wis": "Wisdom", "cha": "Charisma",
}

// AbilityIndex returns the position of an ability key, or -1.
func AbilityIndex(key string) int {
	for i, a := range Abilities {
		if a == key {
			return i
		}
	}
	return -1
}

// Skill is one of the 18 standard skills.
type Skill struct {
	Index   string
	Name    string
	Ability string
}

var Skills = []Skill{
	{"acrobatics", "Acrobatics", "dex"},
	{"animal-handling", "Animal Handling", "wis"},
	{"arcana", "Arcana", "int"},
	{"athletics", "Athletics", "str"},
	{"deception", "Deception", "cha"},
	{"history", "History", "int"},
	{"insight", "Insight", "wis"},
	{"intimidation", "Intimidation", "cha"},
	{"investigation", "Investigation", "int"},
	{"medicine", "Medicine", "wis"},
	{"nature", "Nature", "int"},
	{"perception", "Perception", "wis"},
	{"performance", "Performance", "cha"},
	{"persuasion", "Persuasion", "cha"},
	{"religion", "Religion", "int"},
	{"sleight-of-hand", "Sleight of Hand", "dex"},
	{"stealth", "Stealth", "dex"},
	{"survival", "Survival", "wis"},
}

// SkillByIndex finds a skill by index.
func SkillByIndex(index string) (Skill, bool) {
	for _, s := range Skills {
		if s.Index == index {
			return s, true
		}
	}
	return Skill{}, false
}

var Alignments = []string{
	"Lawful Good", "Neutral Good", "Chaotic Good",
	"Lawful Neutral", "True Neutral", "Chaotic Neutral",
	"Lawful Evil", "Neutral Evil", "Chaotic Evil", "Unaligned",
}

// ClassEntry is one class a character has levels in.
type ClassEntry struct {
	ID          int64
	Index       string
	Subclass    string
	Level       int
	HitDiceUsed int
}

// Prof is a proficiency. For skills Name is the skill index, for saves the
// ability key. Ref is the equipment or category it covers, when known.
type Prof struct {
	Kind      string
	Name      string
	Ref       string
	Expertise bool
}

// Item is an inventory entry. SRD items are copied in when added, so custom items
// and edits behave the same way.
type Item struct {
	ID                 int64
	SRDIndex           string
	Name               string
	Kind               string // weapon, armor, shield, gear, tool, mount, magic
	Qty                int
	Weight             float64
	Cost               string
	Description        string
	Equipped           bool
	Attuned            bool
	RequiresAttunement bool
	MagicBonus         int
	ACBonus            int
	ArmorCategory      string
	ArmorBase          int
	ArmorDex           bool
	ArmorMaxDex        int
	StrMin             int
	StealthDis         bool
	WeaponCategory     string
	WeaponRange        string
	DamageDice         string
	DamageType         string
	TwoHandedDice      string
	Properties         string
	RangeText          string
	ForceProficient    bool
}

// HasProperty reports whether a weapon has a named property.
func (it Item) HasProperty(name string) bool {
	for _, p := range strings.Split(it.Properties, ",") {
		if strings.EqualFold(strings.TrimSpace(p), name) {
			return true
		}
	}
	return false
}

// IsWeapon reports whether the item can make an attack.
func (it Item) IsWeapon() bool { return it.DamageDice != "" }

// Active reports whether a worn item's bonuses apply: it must be equipped, and
// attuned when it needs attunement.
func (it Item) Active() bool { return it.Equipped && (!it.RequiresAttunement || it.Attuned) }

// Spell is a spell the character knows. SRD spells are copied in when added.
type Spell struct {
	ID             int64
	SRDIndex       string
	Name           string
	Level          int
	School         string
	CastingTime    string
	RangeText      string
	Components     string
	Duration       string
	Description    string
	HigherLevel    string
	Ritual         bool
	Concentration  bool
	SourceClass    string
	Prepared       bool
	AlwaysPrepared bool
	AttackType     string
	SaveAbility    string
	DamageDice     string
	DamageType     string
}

// Feature is a class feature, racial trait, background feature or custom entry.
type Feature struct {
	ID          int64
	SRDIndex    string
	Name        string
	Source      string
	ClassIndex  string
	Level       int
	Description string
	UsesMax     int
	UsesUsed    int
	Recharge    string // short, long or empty
}

// Roll is one entry in a character's roll log. A roll without a total records an
// event such as casting a spell.
type Roll struct {
	ID         int64
	Label      string
	Expression string
	Detail     string
	Total      int
	HasTotal   bool
	CreatedAt  int64
}

// Character is the stored state of a sheet. Everything calculated lives on
// Derived, which Compute builds from this.
type Character struct {
	ID           int64
	UserID       int64
	Owner        string
	Name         string
	RaceIndex    string
	SubraceIndex string
	RaceName     string
	Background   string
	Alignment    string
	Experience   int

	Base  [6]int
	Bonus [6]int

	HPBase         int
	HPPerLevel     int
	HPBonus        int
	HPCurrent      int
	HPTemp         int
	DeathSuccesses int
	DeathFailures  int
	Inspiration    bool
	Exhaustion     int
	Conditions     []string

	Speed      int
	Size       string
	ACBonus    int
	InitBonus  int
	SpeedBonus int
	SaveBonus  int
	PactUsed   int

	CP, SP, EP, GP, PP int

	Age, Height, WeightText, Eyes, Skin, Hair, Faith string
	Personality, Ideals, Bonds, Flaws                string
	Appearance, Backstory, Allies, Notes             string

	CreatedAt int64
	UpdatedAt int64

	Classes       []ClassEntry
	Profs         []Prof
	Items         []Item
	Spells        []Spell
	Features      []Feature
	SlotsUsed     [9]int
	ResourcesUsed map[string]int
}

// HasCondition reports whether a condition is active.
func (c *Character) HasCondition(index string) bool {
	for _, v := range c.Conditions {
		if v == index {
			return true
		}
	}
	return false
}

// HasProf reports whether a proficiency is present.
func (c *Character) HasProf(kind, name string) bool {
	_, ok := c.prof(kind, name)
	return ok
}

func (c *Character) prof(kind, name string) (Prof, bool) {
	for _, p := range c.Profs {
		if p.Kind == kind && p.Name == name {
			return p, true
		}
	}
	return Prof{}, false
}

// AddProf adds a proficiency unless it's already present.
func (c *Character) AddProf(p Prof) {
	if !c.HasProf(p.Kind, p.Name) {
		c.Profs = append(c.Profs, p)
	}
}

// Class finds one of the character's classes by index.
func (c *Character) Class(index string) *ClassEntry {
	for i := range c.Classes {
		if c.Classes[i].Index == index {
			return &c.Classes[i]
		}
	}
	return nil
}

// Level is the total character level across all classes.
func (c *Character) Level() int {
	total := 0
	for _, e := range c.Classes {
		total += e.Level
	}
	return total
}

// Score is an ability score including bonuses from race and improvements.
func (c *Character) Score(i int) int { return c.Base[i] + c.Bonus[i] }

// Mod is an ability's modifier.
func (c *Character) Mod(i int) int { return Modifier(c.Score(i)) }

// ModOf is an ability's modifier by key. Unknown keys give 0.
func (c *Character) ModOf(key string) int {
	if i := AbilityIndex(key); i >= 0 {
		return c.Mod(i)
	}
	return 0
}
