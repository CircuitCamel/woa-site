package sheet

import (
	"errors"
	"fmt"
	"warofages/internal/srd"
)

// Input limits, shared by the handlers and the mutators below.
const (
	MaxAttuned = 3
	MaxLevel   = 20
)

var (
	ErrNoSlot       = errors.New("no spell slot of that level is left")
	ErrNoHitDice    = errors.New("no hit dice of that class are left")
	ErrAttuneLimit  = fmt.Errorf("you can attune to at most %d items", MaxAttuned)
	ErrNoAttunement = errors.New("that item doesn't need attunement")
	ErrMaxLevel     = fmt.Errorf("characters stop at level %d", MaxLevel)
	ErrSlotTooLow   = errors.New("that slot is too low for this spell")
)

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// TakeDamage removes temporary hit points first, then hit points. Damage taken
// while already at 0 counts as a failed death save.
func (c *Character) TakeDamage(amount int) {
	if amount <= 0 {
		return
	}
	if c.HPCurrent <= 0 && c.HPTemp == 0 {
		c.DeathFailures = clamp(c.DeathFailures+1, 0, 3)
		return
	}
	absorbed := min(amount, c.HPTemp)
	c.HPTemp -= absorbed
	c.HPCurrent = max(c.HPCurrent-(amount-absorbed), 0)
}

// Heal restores hit points up to the maximum. Any healing ends dying.
func (c *Character) Heal(amount int) {
	if amount <= 0 {
		return
	}
	c.HPCurrent = clamp(c.HPCurrent+amount, 0, MaxHP(c))
	if c.HPCurrent > 0 {
		c.DeathSuccesses, c.DeathFailures = 0, 0
	}
}

// SetTempHP applies temporary hit points. They don't stack, so the higher wins.
func (c *Character) SetTempHP(amount int) {
	if amount > c.HPTemp {
		c.HPTemp = amount
	}
}

// DeathSave applies a rolled d20: 10 or more succeeds, a natural 1 counts as two
// failures, and a natural 20 brings the character back with 1 hit point.
func (c *Character) DeathSave(natural int) {
	switch {
	case natural == 20:
		c.HPCurrent = 1
		c.DeathSuccesses, c.DeathFailures = 0, 0
	case natural == 1:
		c.DeathFailures = clamp(c.DeathFailures+2, 0, 3)
	case natural >= 10:
		c.DeathSuccesses = clamp(c.DeathSuccesses+1, 0, 3)
	default:
		c.DeathFailures = clamp(c.DeathFailures+1, 0, 3)
	}
}

// SpendHitDie rolls one hit die of a class during a short rest and returns the
// hit points regained (at least 0) along with the roll.
func (c *Character) SpendHitDie(classIndex string) (int, Result, error) {
	e := c.Class(classIndex)
	if e == nil || e.HitDiceUsed >= e.Level {
		return 0, Result{}, ErrNoHitDice
	}
	cl, ok := srd.Get().Class(classIndex)
	if !ok {
		return 0, Result{}, ErrNoHitDice
	}
	res, err := RollDice(dice(fmt.Sprintf("1d%d", cl.HitDie), c.ModOf("con")), false)
	if err != nil {
		return 0, Result{}, err
	}
	e.HitDiceUsed++
	gain := max(res.Total, 0)
	c.Heal(gain)
	return gain, res, nil
}

// ShortRest recharges short-rest features and pact slots. Hit dice are spent
// separately with SpendHitDie.
func (c *Character) ShortRest() {
	c.PactUsed = 0
	for _, r := range computeResources(c) {
		if r.Recharge == "short" {
			delete(c.ResourcesUsed, r.Key)
		}
	}
	for i := range c.Features {
		if c.Features[i].Recharge == "short" {
			c.Features[i].UsesUsed = 0
		}
	}
}

// LongRest restores hit points, half of the total hit dice (at least one), every
// slot and feature, and removes one level of exhaustion.
func (c *Character) LongRest() {
	if c.Exhaustion > 0 {
		c.Exhaustion--
	}
	c.HPCurrent = MaxHP(c)
	c.HPTemp = 0
	c.DeathSuccesses, c.DeathFailures = 0, 0
	c.PactUsed = 0
	c.SlotsUsed = [9]int{}
	c.ResourcesUsed = map[string]int{}
	for i := range c.Features {
		if c.Features[i].Recharge != "" {
			c.Features[i].UsesUsed = 0
		}
	}
	regain := max(c.Level()/2, 1)
	for i := range c.Classes {
		back := min(regain, c.Classes[i].HitDiceUsed)
		c.Classes[i].HitDiceUsed -= back
		regain -= back
	}
}

// UseSlot spends a regular spell slot.
func (c *Character) UseSlot(level int) error {
	slots := SpellSlots(c)
	if level < 1 || level > 9 || c.SlotsUsed[level-1] >= slots[level-1] {
		return ErrNoSlot
	}
	c.SlotsUsed[level-1]++
	return nil
}

// UsePact spends a pact magic slot.
func (c *Character) UsePact() error {
	count, _ := PactSlots(c)
	if c.PactUsed >= count {
		return ErrNoSlot
	}
	c.PactUsed++
	return nil
}

// Cast spends the slot for a spell. Pact slots are cast at their fixed level, and
// cantrips and rituals cost nothing.
func (c *Character) Cast(sp Spell, slotLevel int, pact, ritual bool) (int, error) {
	if sp.Level == 0 || (ritual && sp.Ritual) {
		return sp.Level, nil
	}
	if pact {
		_, level := PactSlots(c)
		if level < sp.Level {
			return 0, ErrSlotTooLow
		}
		return level, c.UsePact()
	}
	if slotLevel < sp.Level {
		return 0, ErrSlotTooLow
	}
	return slotLevel, c.UseSlot(slotLevel)
}

// SetResourceUsed sets how much of a tracked class resource is spent.
func (c *Character) SetResourceUsed(key string, used int) bool {
	for _, r := range computeResources(c) {
		if r.Key == key {
			if c.ResourcesUsed == nil {
				c.ResourcesUsed = map[string]int{}
			}
			c.ResourcesUsed[key] = clamp(used, 0, r.Max)
			return true
		}
	}
	return false
}

// ToggleCondition switches a known condition on or off.
func (c *Character) ToggleCondition(index string) bool {
	if _, ok := srd.Get().Condition(index); !ok {
		return false
	}
	for i, v := range c.Conditions {
		if v == index {
			c.Conditions = append(c.Conditions[:i], c.Conditions[i+1:]...)
			return true
		}
	}
	c.Conditions = append(c.Conditions, index)
	return true
}

// LevelUpInput describes one level gained.
type LevelUpInput struct {
	Class    string
	Subclass string
	HPMethod string // average, roll or manual
	HPManual int
	ASI      [2]string // abilities to raise by one each; the same twice is +2
}

// LevelUp applies one level and returns the hit die gain. Scores are capped at 20
// for improvements, as the rules require.
func (c *Character) LevelUp(in LevelUpInput) (int, error) {
	if c.Level() >= MaxLevel {
		return 0, ErrMaxLevel
	}
	cl, ok := srd.Get().Class(in.Class)
	if !ok {
		return 0, errors.New("unknown class")
	}
	if err := MulticlassError(c, in.Class); err != nil {
		return 0, err
	}
	gain := cl.HitDie/2 + 1
	switch in.HPMethod {
	case "roll":
		gain = Die(cl.HitDie)
	case "manual":
		gain = clamp(in.HPManual, 1, cl.HitDie)
	}

	nextLevel := classLevel(c, in.Class) + 1
	var chosen *srd.Subclass
	if in.Subclass != "" {
		sub, ok := cl.Subclass(in.Subclass)
		if !ok || sub.MinLevel() > nextLevel {
			return 0, errors.New("that subclass isn't available at this level")
		}
		chosen = &sub
	}

	entry := c.Class(in.Class)
	if entry == nil {
		c.Classes = append(c.Classes, ClassEntry{Index: in.Class})
		entry = &c.Classes[len(c.Classes)-1]
		for _, p := range cl.MulticlassProfs {
			c.AddProf(Prof{Kind: p.Kind, Name: p.Name, Ref: p.Ref})
		}
	}
	before := MaxHP(c)
	entry.Level++
	c.HPBase += gain
	if chosen != nil {
		entry.Subclass = chosen.Index
	}
	if HasASI(in.Class, entry.Level) {
		for _, key := range in.ASI {
			if i := AbilityIndex(key); i >= 0 && c.Score(i) < 20 {
				c.Bonus[i]++
			}
		}
	}
	// Keep damage already taken; a level adds to both current and maximum.
	c.HPCurrent += MaxHP(c) - before
	return gain, nil
}

// DesiredFeatures lists the SRD class and subclass features the character's
// current levels grant.
func DesiredFeatures(c *Character) []Feature {
	data := srd.Get()
	var out []Feature
	add := func(index, source, class string, level int) {
		f, ok := data.Features[index]
		if !ok {
			return
		}
		out = append(out, Feature{SRDIndex: index, Name: f.Name, Source: source, ClassIndex: class, Level: level, Description: f.Desc})
	}
	for _, e := range c.Classes {
		cl, ok := data.Class(e.Index)
		if !ok {
			continue
		}
		for n := 1; n <= e.Level; n++ {
			for _, index := range cl.Level(n).Features {
				add(index, cl.Name, e.Index, n)
			}
		}
		if sub, ok := cl.Subclass(e.Subclass); ok {
			for _, l := range sub.Levels {
				if l.Level > e.Level {
					continue
				}
				for _, index := range l.Features {
					add(index, cl.Name+" ("+sub.Name+")", e.Index, l.Level)
				}
			}
		}
	}
	return out
}
