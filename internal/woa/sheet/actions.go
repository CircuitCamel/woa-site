package sheet

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	game "warofages/internal/sheet"
	"warofages/internal/srd"
)

const maxAmount = 1000000

func doHP(m *game.Mut, r *http.Request) error {
	amount := clampInt(formInt(r, "amount", 0), 0, maxAmount)
	switch r.PostFormValue("op") {
	case "damage":
		m.C.TakeDamage(amount)
	case "heal":
		m.C.Heal(amount)
	case "temp":
		m.C.SetTempHP(amount)
	default:
		return fail("Choose damage, heal or temporary hit points.")
	}
	return nil
}

func doInspiration(m *game.Mut, r *http.Request) error {
	m.C.Inspiration = !m.C.Inspiration
	return nil
}

func doExhaustion(m *game.Mut, r *http.Request) error {
	m.C.Exhaustion = clampInt(formInt(r, "level", 0), 0, 6)
	return nil
}

func doCondition(m *game.Mut, r *http.Request) error {
	if !m.C.ToggleCondition(r.PostFormValue("condition")) {
		return fail("Unknown condition.")
	}
	return nil
}

func doDeath(m *game.Mut, r *http.Request) error {
	if r.PostFormValue("op") == "reset" {
		m.C.DeathSuccesses, m.C.DeathFailures = 0, 0
		return nil
	}
	natural := game.Die(20)
	m.C.DeathSave(natural)
	return m.LogRoll("Death Save", game.Result{Expression: "1d20", Detail: fmt.Sprintf("[%d]", natural), Total: natural})
}

func doRest(m *game.Mut, r *http.Request) error {
	switch r.PostFormValue("kind") {
	case "short":
		m.C.ShortRest()
		return m.LogEvent("Short Rest", "Short-rest features and pact slots recovered.")
	case "long":
		m.C.LongRest()
		return m.LogEvent("Long Rest", "Hit points, spell slots and features recovered.")
	}
	return fail("Choose a short or long rest.")
}

func doHitDie(m *game.Mut, r *http.Request) error {
	gain, res, err := m.C.SpendHitDie(r.PostFormValue("class"))
	if err != nil {
		return failErr(err)
	}
	return m.LogRoll(fmt.Sprintf("Hit Die (+%d HP)", gain), res)
}

func doSlot(m *game.Mut, r *http.Request) error {
	level := formInt(r, "level", 0)
	if level < 1 || level > 9 {
		return fail("Unknown spell slot.")
	}
	if r.PostFormValue("op") == "restore" {
		if m.C.SlotsUsed[level-1] > 0 {
			m.C.SlotsUsed[level-1]--
		}
		return nil
	}
	return failErr(m.C.UseSlot(level))
}

func doPact(m *game.Mut, r *http.Request) error {
	if r.PostFormValue("op") == "restore" {
		if m.C.PactUsed > 0 {
			m.C.PactUsed--
		}
		return nil
	}
	return failErr(m.C.UsePact())
}

func doResource(m *game.Mut, r *http.Request) error {
	key := r.PostFormValue("key")
	amount := clampInt(formInt(r, "amount", 1), 1, maxAmount)
	if r.PostFormValue("op") == "restore" {
		amount = -amount
	}
	if !m.C.SetResourceUsed(key, m.C.ResourcesUsed[key]+amount) {
		return fail("Unknown resource.")
	}
	return nil
}

// doRoll rolls on the server. The form names what's being rolled, never a
// modifier or total, so values always come from the sheet.
func doRoll(m *game.Mut, r *http.Request) error {
	d := game.Compute(m.C)
	what := r.PostFormValue("what")
	mode := r.PostFormValue("mode")
	kind, key, _ := strings.Cut(what, ":")
	check := func(label string, mod int) error {
		return m.LogRoll(label, game.RollD20(mod, mode))
	}
	switch kind {
	case "check":
		for _, a := range d.Abilities {
			if a.Key == key {
				return check(a.Name+" Check", a.Mod)
			}
		}
	case "save":
		for _, a := range d.Abilities {
			if a.Key == key {
				return check(a.Name+" Save", a.Save)
			}
		}
	case "skill":
		for _, s := range d.Skills {
			if s.Index == key {
				if s.Disadvantage && mode == "" {
					mode = "dis"
				}
				return check(s.Name, s.Bonus)
			}
		}
	case "initiative":
		return check("Initiative", d.Initiative)
	case "attack", "damage", "damage2":
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return fail("Unknown attack.")
		}
		for _, a := range d.Attacks {
			if a.ItemID != id {
				continue
			}
			if kind == "attack" {
				return check(a.Name+" Attack", a.Bonus)
			}
			expr := a.Damage
			if kind == "damage2" && a.TwoHanded != "" {
				expr = a.TwoHanded
			}
			label := strings.TrimSpace(a.Name + " Damage " + a.DamageType)
			if mode == "crit" {
				label = "Critical " + label
			}
			res, err := game.RollDice(expr, mode == "crit")
			if err != nil {
				return failErr(err)
			}
			return m.LogRoll(label, res)
		}
	case "custom":
		res, err := game.RollDice(r.PostFormValue("expr"), false)
		if err != nil {
			return failErr(err)
		}
		label := clip(r.PostFormValue("label"), 60)
		if label == "" {
			label = "Custom Roll"
		}
		return m.LogRoll(label, res)
	}
	return fail("Unknown roll.")
}

func doCurrency(m *game.Mut, r *http.Request) error {
	c := m.C
	for name, field := range map[string]*int{"cp": &c.CP, "sp": &c.SP, "ep": &c.EP, "gp": &c.GP, "pp": &c.PP} {
		*field = clampInt(formInt(r, name, *field), 0, maxAmount)
	}
	return nil
}

func casterFor(d *game.Derived, class string) *game.CasterView {
	for i := range d.Casters {
		if d.Casters[i].Class == class {
			return &d.Casters[i]
		}
	}
	if len(d.Casters) > 0 {
		return &d.Casters[0]
	}
	return nil
}

// doCast handles one spell form: cast it, cast it as a ritual, or roll its attack,
// damage or healing at the selected slot level.
func doCast(m *game.Mut, r *http.Request) error {
	sp := m.Spell(formID(r, "spell"))
	if sp == nil {
		return fail("Unknown spell.")
	}
	slotValue := r.PostFormValue("slot")
	pact := slotValue == "pact"
	slot, _ := strconv.Atoi(slotValue)
	if pact {
		_, slot = game.PactSlots(m.C)
	}
	if slot < sp.Level {
		slot = sp.Level
	}
	d := game.Compute(m.C)
	mod := 0
	caster := casterFor(d, sp.SourceClass)
	if caster != nil {
		mod = caster.Mod
	}

	switch r.PostFormValue("op") {
	case "cast", "ritual":
		ritual := r.PostFormValue("op") == "ritual"
		level, err := m.C.Cast(*sp, slot, pact, ritual)
		if err != nil {
			return failErr(err)
		}
		detail := "Cantrip"
		switch {
		case ritual:
			detail = "As a ritual, no slot used"
		case sp.Level > 0:
			detail = "Level " + strconv.Itoa(level) + " slot"
		}
		return m.LogEvent("Cast "+sp.Name, detail)
	case "attack":
		if caster == nil {
			return fail("This character has no spellcasting class.")
		}
		return m.LogRoll(sp.Name+" Attack", game.RollD20(caster.Attack, r.PostFormValue("mode")))
	case "damage":
		expr, typ := game.SpellDamage(*sp, slot, d.Level, mod)
		if expr == "" {
			return fail("This spell has no damage dice.")
		}
		res, err := game.RollDice(expr, false)
		if err != nil {
			return failErr(err)
		}
		return m.LogRoll(strings.TrimSpace(sp.Name+" Damage "+typ), res)
	case "heal":
		expr := game.SpellHealing(*sp, slot, mod)
		if expr == "" {
			return fail("This spell has no healing dice.")
		}
		res, err := game.RollDice(expr, false)
		if err != nil {
			return failErr(err)
		}
		return m.LogRoll(sp.Name+" Healing", res)
	}
	return fail("Unknown spell action.")
}

func doSpellAdd(m *game.Mut, r *http.Request) error {
	s, ok := srd.Get().Spell(r.PostFormValue("srd"))
	if !ok {
		return fail("Unknown spell.")
	}
	source := r.PostFormValue("source_class")
	if m.C.Class(source) == nil {
		source = defaultSource(m.C, s)
	}
	sp := game.SpellFromSRD(*s, source)
	sp.Prepared = !preparesSpells(source) || s.Level == 0
	return m.AddSpell(sp)
}

func preparesSpells(class string) bool {
	cl, ok := srd.Get().Class(class)
	return ok && cl.Prepares
}

func doSpellCustom(m *game.Mut, r *http.Request) error {
	name := clip(r.PostFormValue("name"), 100)
	if name == "" {
		return fail("Give the spell a name.")
	}
	dice := clip(r.PostFormValue("damage_dice"), 40)
	if dice != "" {
		if err := game.ValidateDice(dice); err != nil {
			return failErr(err)
		}
	}
	source := r.PostFormValue("source_class")
	if m.C.Class(source) == nil {
		source = ""
	}
	return m.AddSpell(game.Spell{
		Name: name, Level: clampInt(formInt(r, "level", 0), 0, 9), School: clip(r.PostFormValue("school"), 40),
		CastingTime: clip(r.PostFormValue("casting_time"), 60), RangeText: clip(r.PostFormValue("range"), 60),
		Components: clip(r.PostFormValue("components"), 200), Duration: clip(r.PostFormValue("duration"), 60),
		Description: clip(r.PostFormValue("description"), textLimit), SourceClass: source, Prepared: true,
		Ritual: r.PostFormValue("ritual") != "", Concentration: r.PostFormValue("concentration") != "",
		DamageDice: dice, DamageType: clip(r.PostFormValue("damage_type"), 40),
	})
}

func doSpellPrepare(m *game.Mut, r *http.Request) error {
	sp := m.Spell(formID(r, "spell"))
	if sp == nil {
		return fail("Unknown spell.")
	}
	if sp.AlwaysPrepared {
		return nil
	}
	return m.SetSpellPrepared(sp.ID, !sp.Prepared)
}

func doSpellDelete(m *game.Mut, r *http.Request) error { return m.DeleteSpell(formID(r, "spell")) }

func doItemAdd(m *game.Mut, r *http.Request) error {
	s, ok := srd.Get().Item(r.PostFormValue("srd"))
	if !ok {
		return fail("Unknown item.")
	}
	it := game.ItemFromSRD(*s)
	it.Qty = clampInt(formInt(r, "qty", 1), 1, maxAmount)
	return m.AddItem(it)
}

var itemKinds = map[string]bool{"weapon": true, "armor": true, "shield": true, "gear": true, "tool": true, "mount": true, "magic": true}

// readItem applies the item form. Shared by adding a custom item and editing.
func readItem(r *http.Request, it *game.Item) error {
	name := clip(r.PostFormValue("name"), 100)
	if name == "" {
		return fail("Give the item a name.")
	}
	kind := r.PostFormValue("kind")
	if !itemKinds[kind] {
		return fail("Unknown item type.")
	}
	weight, err := strconv.ParseFloat(strings.TrimSpace(r.PostFormValue("weight")), 64)
	if err != nil || math.IsNaN(weight) || math.IsInf(weight, 0) {
		weight = 0
	}
	for _, field := range []string{"damage_dice", "two_handed_dice"} {
		if v := strings.TrimSpace(r.PostFormValue(field)); v != "" {
			if err := game.ValidateDice(v); err != nil {
				return failErr(err)
			}
		}
	}
	it.Name, it.Kind = name, kind
	it.Qty = clampInt(formInt(r, "qty", 1), 0, maxAmount)
	it.Weight = max(0, min(weight, maxAmount))
	it.Cost = clip(r.PostFormValue("cost"), 40)
	it.Description = clip(r.PostFormValue("description"), textLimit)
	it.RequiresAttunement = r.PostFormValue("requires_attunement") != ""
	if !it.RequiresAttunement {
		it.Attuned = false
	}
	it.MagicBonus = clampInt(formInt(r, "magic_bonus", 0), -20, 20)
	it.ACBonus = clampInt(formInt(r, "ac_bonus", 0), -20, 20)
	it.ArmorCategory = clip(r.PostFormValue("armor_category"), 20)
	it.ArmorBase = clampInt(formInt(r, "armor_base", 0), 0, 40)
	it.ArmorDex = r.PostFormValue("armor_dex") != ""
	it.ArmorMaxDex = clampInt(formInt(r, "armor_max_dex", 0), 0, 20)
	it.StrMin = clampInt(formInt(r, "str_min", 0), 0, 30)
	it.StealthDis = r.PostFormValue("stealth_dis") != ""
	it.WeaponCategory = clip(r.PostFormValue("weapon_category"), 20)
	it.WeaponRange = clip(r.PostFormValue("weapon_range"), 20)
	it.DamageDice = clip(r.PostFormValue("damage_dice"), 40)
	it.DamageType = clip(r.PostFormValue("damage_type"), 40)
	it.TwoHandedDice = clip(r.PostFormValue("two_handed_dice"), 40)
	it.Properties = clip(r.PostFormValue("properties"), 200)
	it.RangeText = clip(r.PostFormValue("range_text"), 40)
	it.ForceProficient = r.PostFormValue("force_proficient") != ""
	return nil
}

func doItemCustom(m *game.Mut, r *http.Request) error {
	it := game.Item{}
	if err := readItem(r, &it); err != nil {
		return err
	}
	return m.AddItem(it)
}

func doItemEdit(m *game.Mut, r *http.Request) error {
	it := m.Item(formID(r, "item"))
	if it == nil {
		return fail("Unknown item.")
	}
	if err := readItem(r, it); err != nil {
		return err
	}
	return m.SaveItem(it)
}

// doItemEquip toggles an item. Only one suit of armor and one shield can be worn,
// so equipping one takes off the other.
func doItemEquip(m *game.Mut, r *http.Request) error {
	it := m.Item(formID(r, "item"))
	if it == nil {
		return fail("Unknown item.")
	}
	it.Equipped = !it.Equipped
	if it.Equipped && (it.Kind == "armor" || it.Kind == "shield") {
		for i := range m.C.Items {
			other := &m.C.Items[i]
			if other.ID != it.ID && other.Kind == it.Kind && other.Equipped {
				other.Equipped = false
				if err := m.SaveItem(other); err != nil {
					return err
				}
			}
		}
	}
	return m.SaveItem(it)
}

func doItemAttune(m *game.Mut, r *http.Request) error {
	it := m.Item(formID(r, "item"))
	if it == nil {
		return fail("Unknown item.")
	}
	if !it.RequiresAttunement {
		return failErr(game.ErrNoAttunement)
	}
	if !it.Attuned && game.Compute(m.C).Attuned >= game.MaxAttuned {
		return failErr(game.ErrAttuneLimit)
	}
	it.Attuned = !it.Attuned
	return m.SaveItem(it)
}

func doItemDelete(m *game.Mut, r *http.Request) error { return m.DeleteItem(formID(r, "item")) }

func doFeatureAdd(m *game.Mut, r *http.Request) error {
	name := clip(r.PostFormValue("name"), 100)
	if name == "" {
		return fail("Give the feature a name.")
	}
	recharge := r.PostFormValue("recharge")
	if recharge != "short" && recharge != "long" {
		recharge = ""
	}
	return m.AddFeature(game.Feature{
		Name: name, Source: "Custom", Description: clip(r.PostFormValue("description"), textLimit),
		UsesMax: clampInt(formInt(r, "uses_max", 0), 0, maxAmount), Recharge: recharge,
	})
}

func doFeatureUse(m *game.Mut, r *http.Request) error {
	f := m.Feature(formID(r, "feature"))
	if f == nil || f.UsesMax == 0 {
		return fail("Unknown feature.")
	}
	if r.PostFormValue("op") == "restore" {
		f.UsesUsed = max(f.UsesUsed-1, 0)
		return nil
	}
	if f.UsesUsed >= f.UsesMax {
		return fail(f.Name + " has no uses left.")
	}
	f.UsesUsed++
	return nil
}

func doFeatureDelete(m *game.Mut, r *http.Request) error {
	return m.DeleteFeature(formID(r, "feature"))
}

// defaultSource picks which class a new spell belongs to: the first class whose
// list has it, otherwise the first spellcasting class.
func defaultSource(c *game.Character, s *srd.Spell) string {
	first := ""
	for _, e := range c.Classes {
		cl, ok := srd.Get().Class(e.Index)
		if !ok || cl.SpellAbility == "" {
			continue
		}
		if first == "" {
			first = e.Index
		}
		for _, listed := range s.Classes {
			if listed == e.Index {
				return e.Index
			}
		}
	}
	return first
}
