package sheet

import (
	"database/sql"
	"path/filepath"
	"testing"
	"warofages/internal/db"
)

func testStore(t *testing.T) (*Store, int64, int64) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &Store{DB: conn}, addUser(t, conn, "alice"), addUser(t, conn, "bobby")
}

func addUser(t *testing.T, conn *sql.DB, name string) int64 {
	t.Helper()
	res, err := conn.Exec(`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, 'x', 'player', 0)`, name)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func clericDraft() Draft {
	d := DefaultDraft()
	d.Name = "Aster"
	d.Class = "cleric"
	d.Subclass = "life"
	d.Race = "dwarf/hill-dwarf"
	d.Skills = []string{"medicine", "persuasion"}
	return d
}

func TestBuildAppliesRaceClassAndBackground(t *testing.T) {
	c, _, err := clericDraft().Build()
	if err != nil {
		t.Fatal(err)
	}
	d := Compute(c)
	// Base 13 con + 2 dwarf = 15 (+2); level 1 cleric d8 + 2 con + 1 toughness.
	if d.MaxHP != 11 || c.HPCurrent != 11 {
		t.Errorf("max HP %d current %d", d.MaxHP, c.HPCurrent)
	}
	if c.Speed != 25 || c.RaceName != "Hill Dwarf" {
		t.Errorf("speed %d race %s", c.Speed, c.RaceName)
	}
	if !c.HasProf("save", "wis") || !c.HasProf("skill", "insight") || !c.HasProf("skill", "medicine") || !c.HasProf("language", "Dwarvish") {
		t.Errorf("profs missing: %+v", c.Profs)
	}
	if c.GP != 15 {
		t.Errorf("background gold %d", c.GP)
	}
	names := map[string]bool{}
	for _, f := range c.Features {
		names[f.SRDIndex] = true
	}
	for _, want := range []string{"race:darkvision", "race:dwarven-toughness", "background:acolyte", "spellcasting-cleric"} {
		if !names[want] {
			t.Errorf("missing feature %s", want)
		}
	}
	always := 0
	for _, sp := range c.Spells {
		if sp.AlwaysPrepared {
			always++
		}
	}
	if always != 2 {
		t.Errorf("life domain level 1 grants 2 spells, got %d", always)
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	bad := []func(*Draft){
		func(d *Draft) { d.Name = "  " },
		func(d *Draft) { d.Class = "nope" },
		func(d *Draft) { d.Race = "nope" },
		func(d *Draft) { d.Race = "dwarf/nope" },
		func(d *Draft) { d.Subclass = "champion" },
		func(d *Draft) { d.Level = 21 },
		func(d *Draft) { d.Scores = [6]int{18, 18, 18, 18, 18, 18} },
		func(d *Draft) { d.Class, d.Subclass = "fighter", "champion" },
		func(d *Draft) { d.Background = "nope" },
	}
	for i, change := range bad {
		d := clericDraft()
		change(&d)
		if _, _, err := d.Build(); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestHalfElfBonusChoices(t *testing.T) {
	d := DefaultDraft()
	d.Name = "Sam"
	d.Race = "half-elf"
	d.BonusChoices = []string{"str", "str", "cha", "dex", "con"} // duplicates, invalid and extra picks ignored
	c, _, err := d.Build()
	if err != nil {
		t.Fatal(err)
	}
	if c.Bonus[0] != 1 || c.Bonus[1] != 1 || c.Bonus[2] != 0 || c.Bonus[5] != 2 {
		t.Errorf("bonuses %v", c.Bonus)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s, alice, _ := testStore(t)
	c, _, err := clericDraft().Build()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Create(c, alice)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(id, Access{UserID: alice})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Aster" || got.Owner != "alice" || got.Bonus != c.Bonus || got.Base != c.Base || got.GP != 15 {
		t.Errorf("scalars: %+v", got)
	}
	if len(got.Classes) != 1 || got.Classes[0].Subclass != "life" || len(got.Features) != len(c.Features) || len(got.Spells) != len(c.Spells) || len(got.Profs) != len(c.Profs) {
		t.Errorf("children: %d %d %d", len(got.Features), len(got.Spells), len(got.Profs))
	}
	if Compute(got).MaxHP != Compute(c).MaxHP {
		t.Error("derived differs after reload")
	}
}

func TestAccessScoping(t *testing.T) {
	s, alice, bobby := testStore(t)
	c, _, _ := clericDraft().Build()
	id, err := s.Create(c, alice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(id, Access{UserID: bobby}); err != ErrNotFound {
		t.Errorf("get by other player: %v", err)
	}
	if err := s.Update(id, Access{UserID: bobby}, func(*Mut) error { return nil }); err != ErrNotFound {
		t.Errorf("update by other player: %v", err)
	}
	if err := s.Delete(id, Access{UserID: bobby}); err != ErrNotFound {
		t.Errorf("delete by other player: %v", err)
	}
	if _, err := s.Get(id, Access{UserID: bobby, IsAdmin: true}); err != nil {
		t.Errorf("admin should have access: %v", err)
	}
	mine, _ := s.List(bobby)
	if len(mine) != 0 {
		t.Errorf("bobby sees %d characters", len(mine))
	}
	all, _ := s.List(0)
	if len(all) != 1 || all[0].Classes != "Cleric 1" || all[0].Owner != "alice" {
		t.Errorf("list all: %+v", all)
	}
}

func TestUpdatePersistsAndRollsBack(t *testing.T) {
	s, alice, _ := testStore(t)
	c, _, _ := clericDraft().Build()
	id, _ := s.Create(c, alice)
	access := Access{UserID: alice}

	err := s.Update(id, access, func(m *Mut) error {
		m.C.HPCurrent = 3
		m.C.ToggleCondition("poisoned")
		m.C.Inspiration = true
		if err := m.C.UseSlot(1); err != nil {
			return err
		}
		m.C.ResourcesUsed["x"] = 1
		return m.LogEvent("Test", "ok")
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(id, access)
	if got.HPCurrent != 3 || !got.HasCondition("poisoned") || !got.Inspiration || got.SlotsUsed[0] != 1 {
		t.Errorf("not persisted: %+v", got)
	}

	err = s.Update(id, access, func(m *Mut) error {
		m.C.HPCurrent = 99
		return ErrNoSlot
	})
	if err != ErrNoSlot {
		t.Fatalf("err = %v", err)
	}
	got, _ = s.Get(id, access)
	if got.HPCurrent != 3 {
		t.Errorf("failed update must not save: %d", got.HPCurrent)
	}
}

func TestLevelUpResyncsFeatures(t *testing.T) {
	s, alice, _ := testStore(t)
	d := DefaultDraft()
	d.Name = "Max"
	d.Class = "fighter"
	d.Level = 2
	c, _, err := d.Build()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Create(c, alice)
	if err != nil {
		t.Fatal(err)
	}
	access := Access{UserID: alice}
	err = s.Update(id, access, func(m *Mut) error {
		_, err := m.C.LevelUp(LevelUpInput{Class: "fighter", Subclass: "champion", HPMethod: "average"})
		m.Resync = true
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(id, access)
	found := false
	for _, f := range got.Features {
		found = found || f.SRDIndex == "improved-critical"
	}
	if !found || got.Level() != 3 {
		t.Errorf("level %d champion feature %v", got.Level(), found)
	}

	// Lowering the level removes features above it but keeps racial ones.
	err = s.Update(id, access, func(m *Mut) error {
		m.C.Classes[0].Level = 1
		m.C.Classes[0].Subclass = ""
		m.Resync = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(id, access)
	for _, f := range got.Features {
		if f.SRDIndex == "improved-critical" || f.SRDIndex == "action-surge-1-use" {
			t.Errorf("%s should be gone", f.SRDIndex)
		}
	}
	if len(got.Features) == 0 {
		t.Error("kept features missing")
	}
}

func TestItemsSpellsFeaturesStayOnTheirCharacter(t *testing.T) {
	s, alice, bobby := testStore(t)
	a, _, _ := clericDraft().Build()
	aliceID, _ := s.Create(a, alice)
	b, _, _ := clericDraft().Build()
	bobbyID, _ := s.Create(b, bobby)

	var bobItem int64
	s.Update(bobbyID, Access{UserID: bobby}, func(m *Mut) error {
		return m.AddItem(Item{Name: "Bob's Sword", Kind: "weapon", Qty: 1})
	})
	got, _ := s.Get(bobbyID, Access{UserID: bobby})
	for _, it := range got.Items {
		if it.Name == "Bob's Sword" {
			bobItem = it.ID
		}
	}

	// Alice can't reach another character's rows by guessing IDs.
	s.Update(aliceID, Access{UserID: alice}, func(m *Mut) error {
		if m.Item(bobItem) != nil {
			t.Error("foreign item visible")
		}
		return m.DeleteItem(bobItem)
	})
	got, _ = s.Get(bobbyID, Access{UserID: bobby})
	found := false
	for _, it := range got.Items {
		found = found || it.ID == bobItem
	}
	if !found {
		t.Error("foreign item deleted")
	}
}

func TestRollLogIsTrimmed(t *testing.T) {
	s, alice, _ := testStore(t)
	c, _, _ := clericDraft().Build()
	id, _ := s.Create(c, alice)
	s.Update(id, Access{UserID: alice}, func(m *Mut) error {
		for i := 0; i < maxRollsKept+5; i++ {
			if err := m.LogRoll("Test", Result{Total: i}); err != nil {
				return err
			}
		}
		return nil
	})
	rolls, err := s.Rolls(id, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(rolls) != maxRollsKept || rolls[0].Total != maxRollsKept+4 {
		t.Errorf("rolls %d newest %d", len(rolls), rolls[0].Total)
	}
}
