package sheet

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

// ErrNotFound means the character doesn't exist or the user can't access it. The
// two are deliberately the same, so other players' characters aren't revealed.
var ErrNotFound = errors.New("character not found")

const maxRollsKept = 200

// Store reads and writes characters.
type Store struct {
	DB *sql.DB
}

// Access says who is asking for a character.
type Access struct {
	UserID  int64
	IsAdmin bool
}

type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

// Summary is a row in a character list.
type Summary struct {
	ID        int64
	UserID    int64
	Owner     string
	Name      string
	RaceName  string
	Level     int
	HPCurrent int
	Classes   string
	UpdatedAt int64
}

// characterColumns pairs each scalar column with its field, in one place, so that
// reads and writes can't drift apart.
func characterColumns(c *Character, conditions *string) ([]string, []any) {
	pairs := []struct {
		col string
		ptr any
	}{
		{"user_id", &c.UserID}, {"name", &c.Name}, {"race_index", &c.RaceIndex}, {"subrace_index", &c.SubraceIndex},
		{"race_name", &c.RaceName}, {"background", &c.Background}, {"alignment", &c.Alignment}, {"experience", &c.Experience},
		{"base_str", &c.Base[0]}, {"base_dex", &c.Base[1]}, {"base_con", &c.Base[2]}, {"base_int", &c.Base[3]}, {"base_wis", &c.Base[4]}, {"base_cha", &c.Base[5]},
		{"bonus_str", &c.Bonus[0]}, {"bonus_dex", &c.Bonus[1]}, {"bonus_con", &c.Bonus[2]}, {"bonus_int", &c.Bonus[3]}, {"bonus_wis", &c.Bonus[4]}, {"bonus_cha", &c.Bonus[5]},
		{"hp_base", &c.HPBase}, {"hp_per_level", &c.HPPerLevel}, {"hp_bonus", &c.HPBonus}, {"hp_current", &c.HPCurrent}, {"hp_temp", &c.HPTemp},
		{"death_successes", &c.DeathSuccesses}, {"death_failures", &c.DeathFailures}, {"inspiration", &c.Inspiration}, {"exhaustion", &c.Exhaustion},
		{"conditions", conditions}, {"speed", &c.Speed}, {"size", &c.Size}, {"ac_bonus", &c.ACBonus}, {"init_bonus", &c.InitBonus}, {"speed_bonus", &c.SpeedBonus},
		{"save_bonus", &c.SaveBonus}, {"pact_used", &c.PactUsed}, {"cp", &c.CP}, {"sp", &c.SP}, {"ep", &c.EP}, {"gp", &c.GP}, {"pp", &c.PP},
		{"age", &c.Age}, {"height", &c.Height}, {"weight_text", &c.WeightText}, {"eyes", &c.Eyes}, {"skin", &c.Skin}, {"hair", &c.Hair}, {"faith", &c.Faith},
		{"personality", &c.Personality}, {"ideals", &c.Ideals}, {"bonds", &c.Bonds}, {"flaws", &c.Flaws}, {"appearance", &c.Appearance},
		{"backstory", &c.Backstory}, {"allies", &c.Allies}, {"notes", &c.Notes}, {"created_at", &c.CreatedAt}, {"updated_at", &c.UpdatedAt},
	}
	cols := make([]string, len(pairs))
	ptrs := make([]any, len(pairs))
	for i, p := range pairs {
		cols[i], ptrs[i] = p.col, p.ptr
	}
	return cols, ptrs
}

func (s *Store) load(q querier, id int64, access Access) (*Character, error) {
	c := &Character{ID: id, ResourcesUsed: map[string]int{}}
	var conditions string
	cols, ptrs := characterColumns(c, &conditions)
	query := "SELECT " + strings.Join(cols, ", ") + " FROM characters WHERE id = ?"
	args := []any{id}
	if !access.IsAdmin {
		query += " AND user_id = ?"
		args = append(args, access.UserID)
	}
	if err := q.QueryRow(query, args...).Scan(ptrs...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if conditions != "" {
		c.Conditions = strings.Split(conditions, ",")
	}
	if err := q.QueryRow(`SELECT username FROM users WHERE id = ?`, c.UserID).Scan(&c.Owner); err != nil {
		return nil, err
	}
	for _, step := range []func(querier, *Character) error{loadClasses, loadProfs, loadItems, loadSpells, loadFeatures, loadSlots, loadResources} {
		if err := step(q, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func loadClasses(q querier, c *Character) error {
	rows, err := q.Query(`SELECT id, class_index, subclass_index, level, hit_dice_used FROM character_classes WHERE character_id = ? ORDER BY position, id`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e ClassEntry
		if err := rows.Scan(&e.ID, &e.Index, &e.Subclass, &e.Level, &e.HitDiceUsed); err != nil {
			return err
		}
		c.Classes = append(c.Classes, e)
	}
	return rows.Err()
}

func loadProfs(q querier, c *Character) error {
	rows, err := q.Query(`SELECT kind, name, ref, expertise FROM character_proficiencies WHERE character_id = ? ORDER BY kind, name`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p Prof
		if err := rows.Scan(&p.Kind, &p.Name, &p.Ref, &p.Expertise); err != nil {
			return err
		}
		c.Profs = append(c.Profs, p)
	}
	return rows.Err()
}

const itemColumns = `srd_index, name, kind, qty, weight, cost, description, equipped, attuned, requires_attunement, magic_bonus, ac_bonus,
	armor_category, armor_base, armor_dex, armor_max_dex, str_min, stealth_dis, weapon_category, weapon_range, damage_dice, damage_type,
	two_handed_dice, properties, range_text, force_proficient`

func itemFields(it *Item) []any {
	return []any{&it.SRDIndex, &it.Name, &it.Kind, &it.Qty, &it.Weight, &it.Cost, &it.Description, &it.Equipped, &it.Attuned, &it.RequiresAttunement,
		&it.MagicBonus, &it.ACBonus, &it.ArmorCategory, &it.ArmorBase, &it.ArmorDex, &it.ArmorMaxDex, &it.StrMin, &it.StealthDis,
		&it.WeaponCategory, &it.WeaponRange, &it.DamageDice, &it.DamageType, &it.TwoHandedDice, &it.Properties, &it.RangeText, &it.ForceProficient}
}

func loadItems(q querier, c *Character) error {
	rows, err := q.Query(`SELECT id, `+itemColumns+` FROM character_items WHERE character_id = ? ORDER BY name, id`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var it Item
		if err := rows.Scan(append([]any{&it.ID}, itemFields(&it)...)...); err != nil {
			return err
		}
		c.Items = append(c.Items, it)
	}
	return rows.Err()
}

const spellColumns = `srd_index, name, level, school, casting_time, range_text, components, duration, description, higher_level, ritual,
	concentration, source_class, prepared, always_prepared, attack_type, save_ability, damage_dice, damage_type`

func spellFields(sp *Spell) []any {
	return []any{&sp.SRDIndex, &sp.Name, &sp.Level, &sp.School, &sp.CastingTime, &sp.RangeText, &sp.Components, &sp.Duration, &sp.Description,
		&sp.HigherLevel, &sp.Ritual, &sp.Concentration, &sp.SourceClass, &sp.Prepared, &sp.AlwaysPrepared, &sp.AttackType, &sp.SaveAbility, &sp.DamageDice, &sp.DamageType}
}

func loadSpells(q querier, c *Character) error {
	rows, err := q.Query(`SELECT id, `+spellColumns+` FROM character_spells WHERE character_id = ? ORDER BY level, name, id`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sp Spell
		if err := rows.Scan(append([]any{&sp.ID}, spellFields(&sp)...)...); err != nil {
			return err
		}
		c.Spells = append(c.Spells, sp)
	}
	return rows.Err()
}

const featureColumns = `srd_index, name, source, class_index, level, description, uses_max, uses_used, recharge`

func featureFields(f *Feature) []any {
	return []any{&f.SRDIndex, &f.Name, &f.Source, &f.ClassIndex, &f.Level, &f.Description, &f.UsesMax, &f.UsesUsed, &f.Recharge}
}

func loadFeatures(q querier, c *Character) error {
	rows, err := q.Query(`SELECT id, `+featureColumns+` FROM character_features WHERE character_id = ? ORDER BY class_index = '', level, name, id`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f Feature
		if err := rows.Scan(append([]any{&f.ID}, featureFields(&f)...)...); err != nil {
			return err
		}
		c.Features = append(c.Features, f)
	}
	return rows.Err()
}

func loadSlots(q querier, c *Character) error {
	rows, err := q.Query(`SELECT level, used FROM character_spell_slots WHERE character_id = ?`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var level, used int
		if err := rows.Scan(&level, &used); err != nil {
			return err
		}
		if level >= 1 && level <= 9 {
			c.SlotsUsed[level-1] = used
		}
	}
	return rows.Err()
}

func loadResources(q querier, c *Character) error {
	rows, err := q.Query(`SELECT key, used FROM character_resources WHERE character_id = ?`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var used int
		if err := rows.Scan(&key, &used); err != nil {
			return err
		}
		c.ResourcesUsed[key] = used
	}
	return rows.Err()
}

// Get loads a full character the asker may access.
func (s *Store) Get(id int64, access Access) (*Character, error) {
	return s.load(s.DB, id, access)
}

// Create saves a character built by Draft.Build for a user and returns its ID.
func (s *Store) Create(c *Character, userID int64) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	c.UserID, c.CreatedAt, c.UpdatedAt = userID, now, now
	conditions := strings.Join(c.Conditions, ",")
	cols, ptrs := characterColumns(c, &conditions)
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	res, err := tx.Exec("INSERT INTO characters ("+strings.Join(cols, ", ")+") VALUES ("+marks+")", ptrs...)
	if err != nil {
		return 0, err
	}
	c.ID, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := saveClasses(tx, c); err != nil {
		return 0, err
	}
	if err := saveProfs(tx, c); err != nil {
		return 0, err
	}
	for i := range c.Items {
		if err := insertItem(tx, c.ID, &c.Items[i]); err != nil {
			return 0, err
		}
	}
	for i := range c.Features {
		if err := insertFeature(tx, c.ID, &c.Features[i]); err != nil {
			return 0, err
		}
	}
	for i := range c.Spells {
		if err := insertSpell(tx, c.ID, &c.Spells[i]); err != nil {
			return 0, err
		}
	}
	return c.ID, tx.Commit()
}

func saveClasses(tx *sql.Tx, c *Character) error {
	if _, err := tx.Exec(`DELETE FROM character_classes WHERE character_id = ?`, c.ID); err != nil {
		return err
	}
	for i, e := range c.Classes {
		if _, err := tx.Exec(`INSERT INTO character_classes (character_id, class_index, subclass_index, level, hit_dice_used, position) VALUES (?, ?, ?, ?, ?, ?)`,
			c.ID, e.Index, e.Subclass, e.Level, e.HitDiceUsed, i); err != nil {
			return err
		}
	}
	return nil
}

func saveProfs(tx *sql.Tx, c *Character) error {
	if _, err := tx.Exec(`DELETE FROM character_proficiencies WHERE character_id = ?`, c.ID); err != nil {
		return err
	}
	for _, p := range c.Profs {
		if _, err := tx.Exec(`INSERT INTO character_proficiencies (character_id, kind, name, ref, expertise) VALUES (?, ?, ?, ?, ?)`,
			c.ID, p.Kind, p.Name, p.Ref, p.Expertise); err != nil {
			return err
		}
	}
	return nil
}

func insertItem(tx *sql.Tx, charID int64, it *Item) error {
	marks := strings.TrimSuffix(strings.Repeat("?, ", 26), ", ")
	res, err := tx.Exec(`INSERT INTO character_items (character_id, `+itemColumns+`) VALUES (?, `+marks+`)`, append([]any{charID}, itemFields(it)...)...)
	if err != nil {
		return err
	}
	it.ID, err = res.LastInsertId()
	return err
}

func insertSpell(tx *sql.Tx, charID int64, sp *Spell) error {
	marks := strings.TrimSuffix(strings.Repeat("?, ", 19), ", ")
	res, err := tx.Exec(`INSERT OR IGNORE INTO character_spells (character_id, `+spellColumns+`) VALUES (?, `+marks+`)`, append([]any{charID}, spellFields(sp)...)...)
	if err != nil {
		return err
	}
	sp.ID, err = res.LastInsertId()
	return err
}

func insertFeature(tx *sql.Tx, charID int64, f *Feature) error {
	marks := strings.TrimSuffix(strings.Repeat("?, ", 9), ", ")
	res, err := tx.Exec(`INSERT OR IGNORE INTO character_features (character_id, `+featureColumns+`) VALUES (?, `+marks+`)`, append([]any{charID}, featureFields(f)...)...)
	if err != nil {
		return err
	}
	f.ID, err = res.LastInsertId()
	return err
}

// Mut is one open transaction on a character. Update saves C's scalar state,
// classes, proficiencies, slots, resources and feature uses when it finishes.
// Items, spells and custom features are changed through the methods below.
type Mut struct {
	C  *Character
	tx *sql.Tx
	// Resync re-derives class features and subclass spells on save. Set it after
	// anything that changes classes or levels.
	Resync bool
}

// Update loads a character, applies fn and saves it, all in one transaction. The
// transaction is opened for writing up front, so concurrent changes run one after
// another rather than overwriting each other.
func (s *Store) Update(id int64, access Access, fn func(*Mut) error) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	c, err := s.load(tx, id, access)
	if err != nil {
		return err
	}
	m := &Mut{C: c, tx: tx}
	if err := fn(m); err != nil {
		return err
	}
	if err := m.save(); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Mut) save() error {
	c := m.C
	c.UpdatedAt = time.Now().Unix()
	conditions := strings.Join(c.Conditions, ",")
	cols, ptrs := characterColumns(c, &conditions)
	sets := make([]string, len(cols))
	for i, col := range cols {
		sets[i] = col + " = ?"
	}
	if _, err := m.tx.Exec("UPDATE characters SET "+strings.Join(sets, ", ")+" WHERE id = ?", append(ptrs, c.ID)...); err != nil {
		return err
	}
	if err := saveClasses(m.tx, c); err != nil {
		return err
	}
	if err := saveProfs(m.tx, c); err != nil {
		return err
	}
	if _, err := m.tx.Exec(`DELETE FROM character_spell_slots WHERE character_id = ?`, c.ID); err != nil {
		return err
	}
	for i, used := range c.SlotsUsed {
		if used > 0 {
			if _, err := m.tx.Exec(`INSERT INTO character_spell_slots (character_id, level, used) VALUES (?, ?, ?)`, c.ID, i+1, used); err != nil {
				return err
			}
		}
	}
	if _, err := m.tx.Exec(`DELETE FROM character_resources WHERE character_id = ?`, c.ID); err != nil {
		return err
	}
	for key, used := range c.ResourcesUsed {
		if used > 0 {
			if _, err := m.tx.Exec(`INSERT INTO character_resources (character_id, key, used) VALUES (?, ?, ?)`, c.ID, key, used); err != nil {
				return err
			}
		}
	}
	for _, f := range c.Features {
		if _, err := m.tx.Exec(`UPDATE character_features SET uses_used = ? WHERE id = ?`, f.UsesUsed, f.ID); err != nil {
			return err
		}
	}
	if m.Resync {
		return m.resync()
	}
	return nil
}

// resync brings SRD class features and subclass spells in line with the current
// levels. Racial, background and custom entries are left alone.
func (m *Mut) resync() error {
	c := m.C
	wanted := map[string]bool{}
	for _, f := range DesiredFeatures(c) {
		wanted[f.SRDIndex] = true
		f := f
		if err := insertFeature(m.tx, c.ID, &f); err != nil {
			return err
		}
	}
	for _, f := range c.Features {
		if f.SRDIndex != "" && f.ClassIndex != "" && !wanted[f.SRDIndex] {
			if _, err := m.tx.Exec(`DELETE FROM character_features WHERE id = ?`, f.ID); err != nil {
				return err
			}
		}
	}
	wantedSpells := map[string]bool{}
	for _, sp := range DesiredSpells(c) {
		wantedSpells[sp.SRDIndex+"|"+sp.SourceClass] = true
		sp := sp
		if err := insertSpell(m.tx, c.ID, &sp); err != nil {
			return err
		}
		if _, err := m.tx.Exec(`UPDATE character_spells SET always_prepared = 1, prepared = 1 WHERE character_id = ? AND srd_index = ? AND source_class = ?`,
			c.ID, sp.SRDIndex, sp.SourceClass); err != nil {
			return err
		}
	}
	for _, sp := range c.Spells {
		if sp.AlwaysPrepared && !wantedSpells[sp.SRDIndex+"|"+sp.SourceClass] {
			if _, err := m.tx.Exec(`DELETE FROM character_spells WHERE id = ?`, sp.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddItem inserts an inventory item, stacking onto an unmodified SRD copy.
func (m *Mut) AddItem(it Item) error {
	if it.SRDIndex != "" {
		for i := range m.C.Items {
			if m.C.Items[i].SRDIndex == it.SRDIndex {
				m.C.Items[i].Qty += it.Qty
				return m.SaveItem(&m.C.Items[i])
			}
		}
	}
	return insertItem(m.tx, m.C.ID, &it)
}

// Item finds an inventory item on this character by ID.
func (m *Mut) Item(id int64) *Item {
	for i := range m.C.Items {
		if m.C.Items[i].ID == id {
			return &m.C.Items[i]
		}
	}
	return nil
}

// SaveItem writes an edited item.
func (m *Mut) SaveItem(it *Item) error {
	sets := make([]string, 0, 26)
	for _, col := range strings.Split(itemColumns, ",") {
		sets = append(sets, strings.TrimSpace(col)+" = ?")
	}
	_, err := m.tx.Exec("UPDATE character_items SET "+strings.Join(sets, ", ")+" WHERE id = ? AND character_id = ?", append(itemFields(it), it.ID, m.C.ID)...)
	return err
}

// DeleteItem removes an item from this character.
func (m *Mut) DeleteItem(id int64) error {
	_, err := m.tx.Exec(`DELETE FROM character_items WHERE id = ? AND character_id = ?`, id, m.C.ID)
	return err
}

// AddSpell adds a spell. Adding the same SRD spell twice for a class does nothing.
func (m *Mut) AddSpell(sp Spell) error { return insertSpell(m.tx, m.C.ID, &sp) }

// Spell finds a known spell on this character by ID.
func (m *Mut) Spell(id int64) *Spell {
	for i := range m.C.Spells {
		if m.C.Spells[i].ID == id {
			return &m.C.Spells[i]
		}
	}
	return nil
}

// SetSpellPrepared stores a spell's prepared state.
func (m *Mut) SetSpellPrepared(id int64, prepared bool) error {
	_, err := m.tx.Exec(`UPDATE character_spells SET prepared = ? WHERE id = ? AND character_id = ?`, prepared, id, m.C.ID)
	return err
}

// DeleteSpell removes a spell from this character. Subclass grants stay.
func (m *Mut) DeleteSpell(id int64) error {
	_, err := m.tx.Exec(`DELETE FROM character_spells WHERE id = ? AND character_id = ? AND always_prepared = 0`, id, m.C.ID)
	return err
}

// AddFeature adds a custom feature.
func (m *Mut) AddFeature(f Feature) error { return insertFeature(m.tx, m.C.ID, &f) }

// Feature finds a feature on this character by ID.
func (m *Mut) Feature(id int64) *Feature {
	for i := range m.C.Features {
		if m.C.Features[i].ID == id {
			return &m.C.Features[i]
		}
	}
	return nil
}

// DeleteFeature removes a custom feature. SRD entries are managed by level.
func (m *Mut) DeleteFeature(id int64) error {
	_, err := m.tx.Exec(`DELETE FROM character_features WHERE id = ? AND character_id = ? AND srd_index = ''`, id, m.C.ID)
	return err
}

// LogRoll adds to the roll log and trims it to the most recent entries.
func (m *Mut) LogRoll(label string, r Result) error {
	return m.logEntry(label, r.Expression, r.Detail, r.Total, true)
}

// LogEvent records something without a roll, such as casting a spell.
func (m *Mut) LogEvent(label, detail string) error {
	return m.logEntry(label, "", detail, 0, false)
}

func (m *Mut) logEntry(label, expr, detail string, total int, hasTotal bool) error {
	if _, err := m.tx.Exec(`INSERT INTO character_rolls (character_id, label, expression, detail, total, has_total, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.C.ID, label, expr, detail, total, hasTotal, time.Now().Unix()); err != nil {
		return err
	}
	_, err := m.tx.Exec(`DELETE FROM character_rolls WHERE character_id = ? AND id NOT IN (SELECT id FROM character_rolls WHERE character_id = ? ORDER BY id DESC LIMIT ?)`,
		m.C.ID, m.C.ID, maxRollsKept)
	return err
}

// Rolls returns a character's log, newest first. The caller must already have
// loaded the character with Get, which is what checks access.
func (s *Store) Rolls(charID int64, limit int) ([]Roll, error) {
	rows, err := s.DB.Query(`SELECT id, label, expression, detail, total, has_total, created_at FROM character_rolls WHERE character_id = ? ORDER BY id DESC LIMIT ?`, charID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Roll
	for rows.Next() {
		var r Roll
		if err := rows.Scan(&r.ID, &r.Label, &r.Expression, &r.Detail, &r.Total, &r.HasTotal, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Delete removes a character the asker may access.
func (s *Store) Delete(id int64, access Access) error {
	query := `DELETE FROM characters WHERE id = ?`
	args := []any{id}
	if !access.IsAdmin {
		query += ` AND user_id = ?`
		args = append(args, access.UserID)
	}
	res, err := s.DB.Exec(query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns summaries for one user, or for everyone when userID is 0.
func (s *Store) List(userID int64) ([]Summary, error) {
	query := `SELECT c.id, c.user_id, u.username, c.name, c.race_name, c.hp_current, c.updated_at,
		COALESCE(cc.class_index, ''), COALESCE(cc.level, 0)
		FROM characters c JOIN users u ON u.id = c.user_id
		LEFT JOIN character_classes cc ON cc.character_id = c.id`
	var args []any
	if userID != 0 {
		query += ` WHERE c.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY c.updated_at DESC, c.id DESC, cc.position`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := map[int64]*Summary{}
	var order []int64
	for rows.Next() {
		var row Summary
		var class string
		var level int
		if err := rows.Scan(&row.ID, &row.UserID, &row.Owner, &row.Name, &row.RaceName, &row.HPCurrent, &row.UpdatedAt, &class, &level); err != nil {
			return nil, err
		}
		got, ok := byID[row.ID]
		if !ok {
			copyRow := row
			byID[row.ID] = &copyRow
			order = append(order, row.ID)
			got = &copyRow
		}
		got.Level += level
		if class != "" {
			if got.Classes != "" {
				got.Classes += " / "
			}
			got.Classes += className(class) + " " + itoa(level)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}
