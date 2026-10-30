package sheet

import (
	"fmt"
	"strconv"
	"strings"
	"warofages/internal/srd"
)

// Modifier is the ability modifier for a score, rounding down.
func Modifier(score int) int {
	if score >= 10 {
		return (score - 10) / 2
	}
	return (score - 11) / 2
}

// ProfBonus is the proficiency bonus at a total character level.
func ProfBonus(level int) int {
	if level < 1 {
		level = 1
	}
	return 2 + (level-1)/4
}

// Derived holds every value calculated from a Character.
type Derived struct {
	Level        int
	ProfBonus    int
	Abilities    []AbilityView
	Skills       []SkillView
	Passives     []Passive
	Initiative   int
	AC           int
	ACDetail     string
	Speed        int
	MaxHP        int
	Carry        int
	Carried      float64
	Encumbered   bool
	HitDice      []HitDieView
	Attacks      []Attack
	Casters      []CasterView
	Slots        []SlotView
	Pact         *PactView
	Resources    []ResourceView
	Warnings     []string
	Attuned      int
	Dying        bool
	Stable       bool
	Dead         bool
	JackOfTrades bool
}

type AbilityView struct {
	Key      string
	Name     string
	Score    int
	Mod      int
	Save     int
	SaveProf bool
}

type SkillView struct {
	Index        string
	Name         string
	Ability      string
	Bonus        int
	Prof         bool
	Expertise    bool
	Half         bool
	Disadvantage bool
}

type Passive struct {
	Name  string
	Value int
}

type HitDieView struct {
	Class     string
	ClassName string
	Die       int
	Total     int
	Used      int
	Remaining int
}

// Attack is a calculated attack line for a weapon or an unarmed strike.
type Attack struct {
	ItemID      int64
	Name        string
	Bonus       int
	Damage      string
	DamageType  string
	TwoHanded   string
	RangeText   string
	Properties  string
	Proficient  bool
	AbilityUsed string
}

type CasterView struct {
	Class        string
	ClassName    string
	Ability      string
	Mod          int
	DC           int
	Attack       int
	Prepares     bool
	PrepareLimit int
	Prepared     int
	Cantrips     int
	Known        int
	CantripsHave int
	KnownHave    int
}

type SlotView struct {
	Level     int
	Max       int
	Used      int
	Remaining int
}

type PactView struct {
	Level     int
	Max       int
	Used      int
	Remaining int
}

type ResourceView struct {
	Key       string
	Name      string
	Max       int
	Used      int
	Remaining int
	Recharge  string
}

// Compute derives every calculated value for a character.
func Compute(c *Character) *Derived {
	d := &Derived{Level: c.Level()}
	d.ProfBonus = ProfBonus(d.Level)
	d.JackOfTrades = classLevel(c, "bard") >= 2
	computeAbilities(c, d)
	armorState := computeAC(c, d)
	computeSkills(c, d, armorState)
	d.Initiative = c.ModOf("dex") + c.InitBonus
	if d.JackOfTrades {
		d.Initiative += d.ProfBonus / 2
	}
	computeSpeed(c, d, armorState)
	d.MaxHP = MaxHP(c)
	computeInventory(c, d)
	computeHitDice(c, d)
	computeAttacks(c, d)
	computeCasting(c, d)
	d.Resources = computeResources(c)
	d.Dead = c.DeathFailures >= 3 || c.Exhaustion >= 6
	d.Stable = !d.Dead && c.HPCurrent <= 0 && c.DeathSuccesses >= 3
	d.Dying = !d.Dead && !d.Stable && c.HPCurrent <= 0 && len(c.Classes) > 0
	return d
}

func classLevel(c *Character, index string) int {
	if e := c.Class(index); e != nil {
		return e.Level
	}
	return 0
}

// MaxHP is the hit point maximum. Hit dice gains are stored without the
// Constitution modifier so that changes to Constitution apply to every level.
func MaxHP(c *Character) int {
	level := c.Level()
	hp := c.HPBase + level*(c.ModOf("con")+c.HPPerLevel) + c.HPBonus
	if hp < level {
		hp = level
	}
	if c.Exhaustion >= 4 {
		hp /= 2
	}
	if hp < 1 {
		hp = 1
	}
	return hp
}

func computeAbilities(c *Character, d *Derived) {
	for i, key := range Abilities {
		mod := c.Mod(i)
		prof := c.HasProf("save", key)
		save := mod + c.SaveBonus
		if prof {
			save += d.ProfBonus
		}
		d.Abilities = append(d.Abilities, AbilityView{
			Key: key, Name: AbilityNames[key], Score: c.Score(i), Mod: mod, Save: save, SaveProf: prof,
		})
	}
}

type armorInfo struct {
	wearing     bool
	heavy       bool
	shield      bool
	stealthDis  bool
	strPenalty  bool
	armorUnprof bool
}

func computeAC(c *Character, d *Derived) armorInfo {
	var info armorInfo
	dex := c.ModOf("dex")
	base := 10 + dex
	detail := fmt.Sprintf("10 + Dex %+d", dex)
	bonus := 0
	var extras []string

	for _, it := range c.Items {
		if !it.Equipped {
			continue
		}
		switch it.Kind {
		case "armor":
			info.wearing = true
			info.heavy = info.heavy || it.ArmorCategory == "Heavy"
			info.stealthDis = info.stealthDis || it.StealthDis
			if it.StrMin > 0 && c.Score(0) < it.StrMin {
				info.strPenalty = true
			}
			if !hasArmorProf(c, it) {
				info.armorUnprof = true
			}
			base = it.ArmorBase + it.MagicBonus
			detail = it.Name + " " + strconv.Itoa(it.ArmorBase)
			if it.MagicBonus != 0 {
				detail += fmt.Sprintf(" %+d", it.MagicBonus)
			}
			if it.ArmorDex {
				add := dex
				if it.ArmorMaxDex > 0 && add > it.ArmorMaxDex {
					add = it.ArmorMaxDex
				}
				base += add
				detail += fmt.Sprintf(" + Dex %+d", add)
			}
		case "shield":
			info.shield = true
			if !hasArmorProf(c, it) {
				info.armorUnprof = true
			}
			bonus += it.ArmorBase + it.MagicBonus
			extras = append(extras, it.Name)
		}
		if it.ACBonus != 0 && it.Active() {
			bonus += it.ACBonus
			extras = append(extras, it.Name)
		}
	}

	if !info.wearing {
		if classLevel(c, "barbarian") > 0 {
			if v := 10 + dex + c.ModOf("con"); v > base {
				base, detail = v, "Unarmored Defense (10 + Dex + Con)"
			}
		}
		if classLevel(c, "monk") > 0 && !info.shield {
			if v := 10 + dex + c.ModOf("wis"); v > base {
				base, detail = v, "Unarmored Defense (10 + Dex + Wis)"
			}
		}
		if e := c.Class("sorcerer"); e != nil && e.Subclass == "draconic" {
			if v := 13 + dex; v > base {
				base, detail = v, "Draconic Resilience (13 + Dex)"
			}
		}
	}

	d.AC = base + bonus + c.ACBonus
	if len(extras) > 0 {
		detail += " + " + strings.Join(extras, " + ")
	}
	if c.ACBonus != 0 {
		detail += fmt.Sprintf(" + other %+d", c.ACBonus)
	}
	d.ACDetail = detail
	if info.armorUnprof {
		d.Warnings = append(d.Warnings, "Wearing armor without proficiency: disadvantage on Strength and Dexterity rolls, and no spellcasting.")
	}
	if info.strPenalty {
		d.Warnings = append(d.Warnings, "Your Strength is too low for this armor: speed reduced by 10 ft.")
	}
	return info
}

func hasArmorProf(c *Character, it Item) bool {
	if it.ForceProficient {
		return true
	}
	want := it.ArmorCategory + " Armor"
	if it.Kind == "shield" {
		want = "Shields"
	}
	for _, p := range c.Profs {
		if p.Kind != "armor" {
			continue
		}
		if strings.EqualFold(p.Name, want) || strings.EqualFold(p.Name, "All armor") && it.Kind == "armor" {
			return true
		}
	}
	return false
}

func hasWeaponProf(c *Character, it Item) bool {
	if it.ForceProficient {
		return true
	}
	for _, p := range c.Profs {
		if p.Kind != "weapon" {
			continue
		}
		if strings.EqualFold(p.Name, it.WeaponCategory+" Weapons") {
			return true
		}
		if p.Ref != "" && p.Ref == it.SRDIndex {
			return true
		}
	}
	return false
}

func computeSkills(c *Character, d *Derived, armor armorInfo) {
	for _, s := range Skills {
		v := SkillView{Index: s.Index, Name: s.Name, Ability: s.Ability, Bonus: c.ModOf(s.Ability)}
		if p, ok := c.prof("skill", s.Index); ok {
			v.Prof = true
			v.Expertise = p.Expertise
			v.Bonus += d.ProfBonus
			if p.Expertise {
				v.Bonus += d.ProfBonus
			}
		} else if d.JackOfTrades {
			v.Half = true
			v.Bonus += d.ProfBonus / 2
		}
		if s.Index == "stealth" && armor.stealthDis {
			v.Disadvantage = true
		}
		d.Skills = append(d.Skills, v)
		switch s.Index {
		case "perception", "investigation", "insight":
			d.Passives = append(d.Passives, Passive{Name: s.Name, Value: 10 + v.Bonus})
		}
	}
}

func computeSpeed(c *Character, d *Derived, armor armorInfo) {
	speed := c.Speed + c.SpeedBonus
	if e := c.Class("monk"); e != nil && !armor.wearing && !armor.shield {
		if cl, ok := srd.Get().Class("monk"); ok {
			speed += cl.Level(e.Level).Specific["unarmored_movement"]
		}
	}
	if classLevel(c, "barbarian") >= 5 && !armor.heavy {
		speed += 10
	}
	if armor.strPenalty {
		speed -= 10
	}
	switch {
	case c.Exhaustion >= 5:
		speed = 0
	case c.Exhaustion >= 2:
		speed /= 2
	}
	if speed < 0 {
		speed = 0
	}
	d.Speed = speed
}

func computeInventory(c *Character, d *Derived) {
	d.Carry = c.Score(0) * 15
	for _, it := range c.Items {
		d.Carried += it.Weight * float64(it.Qty)
		if it.Attuned {
			d.Attuned++
		}
	}
	coins := c.CP + c.SP + c.EP + c.GP + c.PP
	d.Carried += float64(coins) / 50
	d.Encumbered = d.Carried > float64(d.Carry)
	if d.Encumbered {
		d.Warnings = append(d.Warnings, "You're carrying more than your capacity.")
	}
}

func computeHitDice(c *Character, d *Derived) {
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if !ok {
			continue
		}
		d.HitDice = append(d.HitDice, HitDieView{
			Class: e.Index, ClassName: cl.Name, Die: cl.HitDie, Total: e.Level, Used: e.HitDiceUsed, Remaining: e.Level - e.HitDiceUsed,
		})
	}
}

func dice(expr string, mod int) string {
	if mod == 0 {
		return expr
	}
	return fmt.Sprintf("%s%+d", expr, mod)
}

func computeAttacks(c *Character, d *Derived) {
	str, dex := c.ModOf("str"), c.ModOf("dex")
	best := str
	bestName := "str"
	if dex > str {
		best, bestName = dex, "dex"
	}
	monkLevel := classLevel(c, "monk")
	martialDie := 0
	if monkLevel > 0 {
		if cl, ok := srd.Get().Class("monk"); ok {
			martialDie = cl.Level(monkLevel).Specific["martial_arts_die"]
		}
	}

	for _, it := range c.Items {
		if !it.Equipped || !it.IsWeapon() {
			continue
		}
		mod, name := str, "str"
		isMonkWeapon := monkLevel > 0 && it.HasProperty("Monk")
		switch {
		case it.HasProperty("Finesse") || isMonkWeapon:
			mod, name = best, bestName
		case it.WeaponRange == "Ranged":
			mod, name = dex, "dex"
		}
		damage := it.DamageDice
		if isMonkWeapon && dieSize(damage) < martialDie {
			damage = fmt.Sprintf("1d%d", martialDie)
		}
		prof := hasWeaponProf(c, it)
		bonus := mod + it.MagicBonus
		if prof {
			bonus += d.ProfBonus
		}
		a := Attack{
			ItemID: it.ID, Name: it.Name, Bonus: bonus, Damage: dice(damage, mod+it.MagicBonus),
			DamageType: it.DamageType, RangeText: it.RangeText, Properties: it.Properties, Proficient: prof, AbilityUsed: name,
		}
		if it.TwoHandedDice != "" {
			a.TwoHanded = dice(it.TwoHandedDice, mod+it.MagicBonus)
		}
		d.Attacks = append(d.Attacks, a)
	}

	// Unarmed strike is always available.
	unarmed := Attack{Name: "Unarmed Strike", Bonus: str + d.ProfBonus, DamageType: "Bludgeoning", RangeText: "5 ft", Proficient: true, AbilityUsed: "str"}
	if martialDie > 0 {
		unarmed.Bonus = best + d.ProfBonus
		unarmed.AbilityUsed = bestName
		unarmed.Damage = dice(fmt.Sprintf("1d%d", martialDie), best)
	} else {
		flat := 1 + str
		if flat < 0 {
			flat = 0
		}
		unarmed.Damage = strconv.Itoa(flat)
	}
	d.Attacks = append(d.Attacks, unarmed)
}

func dieSize(expr string) int {
	if i := strings.Index(expr, "d"); i >= 0 {
		n, _ := strconv.Atoi(expr[i+1:])
		return n
	}
	return 0
}

// SpellSlots returns the slot table for a character's classes, not counting pact
// magic. One slot-casting class uses its own table. Two or more combine into a
// caster level on the multiclass table, with half casters rounding down.
func SpellSlots(c *Character) [9]int {
	var casters []ClassEntry
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if ok && (cl.Caster == "full" || cl.Caster == "half") {
			casters = append(casters, e)
		}
	}
	switch len(casters) {
	case 0:
		return [9]int{}
	case 1:
		cl, _ := srd.Get().Class(casters[0].Index)
		return cl.Level(casters[0].Level).Slots
	}
	casterLevel := 0
	for _, e := range casters {
		cl, _ := srd.Get().Class(e.Index)
		if cl.Caster == "full" {
			casterLevel += e.Level
		} else {
			casterLevel += e.Level / 2
		}
	}
	if casterLevel == 0 {
		return [9]int{}
	}
	wizard, _ := srd.Get().Class("wizard")
	return wizard.Level(casterLevel).Slots
}

// PactSlots returns the warlock slot count and level.
func PactSlots(c *Character) (count, level int) {
	e := c.Class("warlock")
	if e == nil {
		return 0, 0
	}
	cl, ok := srd.Get().Class("warlock")
	if !ok {
		return 0, 0
	}
	for i, n := range cl.Level(e.Level).Slots {
		if n > 0 {
			return n, i + 1
		}
	}
	return 0, 0
}

func computeCasting(c *Character, d *Derived) {
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if !ok || cl.SpellAbility == "" {
			continue
		}
		row := cl.Level(e.Level)
		mod := c.ModOf(cl.SpellAbility)
		v := CasterView{
			Class: e.Index, ClassName: cl.Name, Ability: cl.SpellAbility, Mod: mod,
			DC: 8 + d.ProfBonus + mod, Attack: d.ProfBonus + mod, Prepares: cl.Prepares,
			Cantrips: row.Cantrips, Known: row.SpellsKnown,
		}
		if cl.Prepares {
			levelPart := e.Level
			if cl.Caster == "half" {
				levelPart = e.Level / 2
			}
			v.PrepareLimit = mod + levelPart
			if v.PrepareLimit < 1 {
				v.PrepareLimit = 1
			}
		}
		for _, s := range c.Spells {
			if s.SourceClass != e.Index {
				continue
			}
			if s.Level == 0 {
				v.CantripsHave++
				continue
			}
			v.KnownHave++
			if s.Prepared && !s.AlwaysPrepared {
				v.Prepared++
			}
		}
		d.Casters = append(d.Casters, v)
	}
	slots := SpellSlots(c)
	for i, max := range slots {
		if max == 0 {
			continue
		}
		used := c.SlotsUsed[i]
		if used > max {
			used = max
		}
		d.Slots = append(d.Slots, SlotView{Level: i + 1, Max: max, Used: used, Remaining: max - used})
	}
	if count, level := PactSlots(c); count > 0 {
		used := c.PactUsed
		if used > count {
			used = count
		}
		d.Pact = &PactView{Level: level, Max: count, Used: used, Remaining: count - used}
	}
}

// Resource keys are stable because used counts are stored against them.
func computeResources(c *Character) []ResourceView {
	var out []ResourceView
	add := func(key, name string, max int, recharge string) {
		if max <= 0 {
			return
		}
		used := c.ResourcesUsed[key]
		if used > max {
			used = max
		}
		out = append(out, ResourceView{Key: key, Name: name, Max: max, Used: used, Remaining: max - used, Recharge: recharge})
	}
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if !ok {
			continue
		}
		spec := cl.Level(e.Level).Specific
		cha := c.ModOf("cha")
		switch e.Index {
		case "barbarian":
			if spec["rage_count"] < 100 { // the table uses 9999 for unlimited at level 20
				add("rage", "Rage", spec["rage_count"], "long")
			}
		case "bard":
			recharge := "long"
			if e.Level >= 5 {
				recharge = "short"
			}
			uses := cha
			if uses < 1 {
				uses = 1
			}
			add("bardic-inspiration", fmt.Sprintf("Bardic Inspiration (d%d)", spec["bardic_inspiration_die"]), uses, recharge)
		case "cleric":
			add("channel-divinity-cleric", "Channel Divinity", spec["channel_divinity_charges"], "short")
		case "druid":
			if e.Level >= 2 && e.Level < 20 {
				add("wild-shape", "Wild Shape", 2, "short")
			}
		case "fighter":
			add("second-wind", "Second Wind", 1, "short")
			add("action-surge", "Action Surge", spec["action_surges"], "short")
			add("indomitable", "Indomitable", spec["indomitable_uses"], "long")
		case "monk":
			add("ki", "Ki Points", spec["ki_points"], "short")
		case "paladin":
			add("divine-sense", "Divine Sense", 1+max(cha, 0), "long")
			add("lay-on-hands", "Lay on Hands (HP pool)", 5*e.Level, "long")
			if e.Level >= 3 {
				add("channel-divinity-paladin", "Channel Divinity", 1, "short")
			}
		case "rogue":
			if e.Level >= 20 {
				add("stroke-of-luck", "Stroke of Luck", 1, "short")
			}
		case "sorcerer":
			add("sorcery-points", "Sorcery Points", spec["sorcery_points"], "long")
		case "wizard":
			add("arcane-recovery", "Arcane Recovery", 1, "long")
		}
	}
	return out
}

// SpellDamage is the dice for a spell at a slot level, or at the character's
// level for cantrips. Custom spells use their own dice.
func SpellDamage(sp Spell, slotLevel, charLevel, mod int) (string, string) {
	if sp.SRDIndex == "" {
		return strings.ReplaceAll(sp.DamageDice, "MOD", strconv.Itoa(mod)), sp.DamageType
	}
	data, ok := srd.Get().Spell(sp.SRDIndex)
	if !ok {
		return "", ""
	}
	if expr := pickScale(data.DamageBySlot, slotLevel); expr != "" {
		return fillMod(expr, mod), data.DamageType
	}
	if expr := pickScale(data.DamageByLevel, charLevel); expr != "" {
		return fillMod(expr, mod), data.DamageType
	}
	return "", ""
}

// SpellHealing is the healing dice for a spell at a slot level.
func SpellHealing(sp Spell, slotLevel, mod int) string {
	if sp.SRDIndex == "" {
		return ""
	}
	data, ok := srd.Get().Spell(sp.SRDIndex)
	if !ok {
		return ""
	}
	return fillMod(pickScale(data.HealBySlot, slotLevel), mod)
}

// pickScale returns the entry for the highest key at or below level.
func pickScale(table map[string]string, level int) string {
	best, bestKey := "", 0
	for k, v := range table {
		n, err := strconv.Atoi(k)
		if err == nil && n <= level && n > bestKey {
			best, bestKey = v, n
		}
	}
	return best
}

func fillMod(expr string, mod int) string {
	if expr == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(expr, " ", ""), "+MOD", fmt.Sprintf("%+d", mod))
}

// Point buy limits.
const (
	PointBuyBudget = 27
	PointBuyMin    = 8
	PointBuyMax    = 15
)

// PointCost returns the cost of a score under point buy.
func PointCost(score int) int {
	switch {
	case score <= 13:
		return score - 8
	case score == 14:
		return 7
	default:
		return 9
	}
}

// ValidateScores checks base scores against the chosen generation method and
// returns the point buy total spent.
func ValidateScores(method string, scores [6]int) (int, error) {
	switch method {
	case "standard":
		want := map[int]int{15: 1, 14: 1, 13: 1, 12: 1, 10: 1, 8: 1}
		for _, s := range scores {
			want[s]--
			if want[s] < 0 {
				return 0, fmt.Errorf("standard array must use 15, 14, 13, 12, 10 and 8 once each")
			}
		}
		return 0, nil
	case "point":
		spent := 0
		for _, s := range scores {
			if s < PointBuyMin || s > PointBuyMax {
				return 0, fmt.Errorf("point buy scores must be between %d and %d", PointBuyMin, PointBuyMax)
			}
			spent += PointCost(s)
		}
		if spent > PointBuyBudget {
			return spent, fmt.Errorf("point buy is over budget: %d of %d points", spent, PointBuyBudget)
		}
		return spent, nil
	default:
		for _, s := range scores {
			if s < 1 || s > 30 {
				return 0, fmt.Errorf("ability scores must be between 1 and 30")
			}
		}
		return 0, nil
	}
}

// MulticlassError explains why a character can't take a level in a class, or
// returns nil. Both the new class and every existing class must meet their
// prerequisites.
func MulticlassError(c *Character, classIndex string) error {
	data := srd.Get()
	target, ok := data.Class(classIndex)
	if !ok {
		return fmt.Errorf("unknown class")
	}
	if c.Class(classIndex) != nil {
		return nil
	}
	if err := checkPrereq(c, target); err != nil {
		return err
	}
	for _, e := range c.Classes {
		if cl, ok := data.Class(e.Index); ok {
			if err := checkPrereq(c, cl); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPrereq(c *Character, cl *srd.Class) error {
	for _, p := range cl.PrereqAll {
		if c.Score(AbilityIndex(p.Ability)) < p.Min {
			return fmt.Errorf("multiclassing with %s requires %s %d", cl.Name, AbilityNames[p.Ability], p.Min)
		}
	}
	if len(cl.PrereqAny) > 0 {
		for _, p := range cl.PrereqAny {
			if c.Score(AbilityIndex(p.Ability)) >= p.Min {
				return nil
			}
		}
		var names []string
		for _, p := range cl.PrereqAny {
			names = append(names, fmt.Sprintf("%s %d", AbilityNames[p.Ability], p.Min))
		}
		return fmt.Errorf("multiclassing with %s requires %s", cl.Name, strings.Join(names, " or "))
	}
	return nil
}

// HasASI reports whether reaching a class level grants an ability score
// improvement.
func HasASI(classIndex string, level int) bool {
	cl, ok := srd.Get().Class(classIndex)
	if !ok || level < 1 || level > 20 {
		return false
	}
	for _, f := range cl.Level(level).Features {
		if strings.Contains(f, "ability-score-improvement") {
			return true
		}
	}
	return false
}

func itoa(n int) string { return strconv.Itoa(n) }

func className(index string) string {
	if cl, ok := srd.Get().Class(index); ok {
		return cl.Name
	}
	return index
}
