// Package db opens the SQLite database and keeps its schema up to date.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open creates the database file if needed, connects and applies migrations.
// The file is created private because it holds password hashes.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	file.Close()

	dsn := url.URL{Scheme: "file", Opaque: url.PathEscape(path), RawQuery: "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"}
	conn, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return conn, nil
}

func migrate(conn *sql.DB) error {
	var version int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// migrations are applied in order and must never be edited once released; add a
// new entry instead.
var migrations = []string{`
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('admin', 'player')),
	must_change_password INTEGER NOT NULL DEFAULT 1,
	disabled INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);

CREATE TABLE sessions (
	token_hash TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE characters (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	race_index TEXT NOT NULL DEFAULT '',
	subrace_index TEXT NOT NULL DEFAULT '',
	race_name TEXT NOT NULL DEFAULT '',
	background TEXT NOT NULL DEFAULT '',
	alignment TEXT NOT NULL DEFAULT '',
	experience INTEGER NOT NULL DEFAULT 0,
	base_str INTEGER NOT NULL DEFAULT 10,
	base_dex INTEGER NOT NULL DEFAULT 10,
	base_con INTEGER NOT NULL DEFAULT 10,
	base_int INTEGER NOT NULL DEFAULT 10,
	base_wis INTEGER NOT NULL DEFAULT 10,
	base_cha INTEGER NOT NULL DEFAULT 10,
	bonus_str INTEGER NOT NULL DEFAULT 0,
	bonus_dex INTEGER NOT NULL DEFAULT 0,
	bonus_con INTEGER NOT NULL DEFAULT 0,
	bonus_int INTEGER NOT NULL DEFAULT 0,
	bonus_wis INTEGER NOT NULL DEFAULT 0,
	bonus_cha INTEGER NOT NULL DEFAULT 0,
	hp_base INTEGER NOT NULL DEFAULT 0,
	hp_per_level INTEGER NOT NULL DEFAULT 0,
	hp_bonus INTEGER NOT NULL DEFAULT 0,
	hp_current INTEGER NOT NULL DEFAULT 0,
	hp_temp INTEGER NOT NULL DEFAULT 0,
	death_successes INTEGER NOT NULL DEFAULT 0,
	death_failures INTEGER NOT NULL DEFAULT 0,
	inspiration INTEGER NOT NULL DEFAULT 0,
	exhaustion INTEGER NOT NULL DEFAULT 0,
	conditions TEXT NOT NULL DEFAULT '',
	speed INTEGER NOT NULL DEFAULT 30,
	size TEXT NOT NULL DEFAULT 'Medium',
	ac_bonus INTEGER NOT NULL DEFAULT 0,
	init_bonus INTEGER NOT NULL DEFAULT 0,
	speed_bonus INTEGER NOT NULL DEFAULT 0,
	save_bonus INTEGER NOT NULL DEFAULT 0,
	pact_used INTEGER NOT NULL DEFAULT 0,
	cp INTEGER NOT NULL DEFAULT 0,
	sp INTEGER NOT NULL DEFAULT 0,
	ep INTEGER NOT NULL DEFAULT 0,
	gp INTEGER NOT NULL DEFAULT 0,
	pp INTEGER NOT NULL DEFAULT 0,
	age TEXT NOT NULL DEFAULT '',
	height TEXT NOT NULL DEFAULT '',
	weight_text TEXT NOT NULL DEFAULT '',
	eyes TEXT NOT NULL DEFAULT '',
	skin TEXT NOT NULL DEFAULT '',
	hair TEXT NOT NULL DEFAULT '',
	faith TEXT NOT NULL DEFAULT '',
	personality TEXT NOT NULL DEFAULT '',
	ideals TEXT NOT NULL DEFAULT '',
	bonds TEXT NOT NULL DEFAULT '',
	flaws TEXT NOT NULL DEFAULT '',
	appearance TEXT NOT NULL DEFAULT '',
	backstory TEXT NOT NULL DEFAULT '',
	allies TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX characters_user ON characters(user_id);

CREATE TABLE character_classes (
	id INTEGER PRIMARY KEY,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	class_index TEXT NOT NULL,
	subclass_index TEXT NOT NULL DEFAULT '',
	level INTEGER NOT NULL,
	hit_dice_used INTEGER NOT NULL DEFAULT 0,
	position INTEGER NOT NULL DEFAULT 0,
	UNIQUE (character_id, class_index)
);

CREATE TABLE character_proficiencies (
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	ref TEXT NOT NULL DEFAULT '',
	expertise INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (character_id, kind, name)
);

CREATE TABLE character_items (
	id INTEGER PRIMARY KEY,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	srd_index TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'gear',
	qty INTEGER NOT NULL DEFAULT 1,
	weight REAL NOT NULL DEFAULT 0,
	cost TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	equipped INTEGER NOT NULL DEFAULT 0,
	attuned INTEGER NOT NULL DEFAULT 0,
	requires_attunement INTEGER NOT NULL DEFAULT 0,
	magic_bonus INTEGER NOT NULL DEFAULT 0,
	ac_bonus INTEGER NOT NULL DEFAULT 0,
	armor_category TEXT NOT NULL DEFAULT '',
	armor_base INTEGER NOT NULL DEFAULT 0,
	armor_dex INTEGER NOT NULL DEFAULT 0,
	armor_max_dex INTEGER NOT NULL DEFAULT 0,
	str_min INTEGER NOT NULL DEFAULT 0,
	stealth_dis INTEGER NOT NULL DEFAULT 0,
	weapon_category TEXT NOT NULL DEFAULT '',
	weapon_range TEXT NOT NULL DEFAULT '',
	damage_dice TEXT NOT NULL DEFAULT '',
	damage_type TEXT NOT NULL DEFAULT '',
	two_handed_dice TEXT NOT NULL DEFAULT '',
	properties TEXT NOT NULL DEFAULT '',
	range_text TEXT NOT NULL DEFAULT '',
	force_proficient INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX character_items_character ON character_items(character_id);

CREATE TABLE character_spells (
	id INTEGER PRIMARY KEY,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	srd_index TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	level INTEGER NOT NULL DEFAULT 0,
	school TEXT NOT NULL DEFAULT '',
	casting_time TEXT NOT NULL DEFAULT '',
	range_text TEXT NOT NULL DEFAULT '',
	components TEXT NOT NULL DEFAULT '',
	duration TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	higher_level TEXT NOT NULL DEFAULT '',
	ritual INTEGER NOT NULL DEFAULT 0,
	concentration INTEGER NOT NULL DEFAULT 0,
	source_class TEXT NOT NULL DEFAULT '',
	prepared INTEGER NOT NULL DEFAULT 0,
	always_prepared INTEGER NOT NULL DEFAULT 0,
	attack_type TEXT NOT NULL DEFAULT '',
	save_ability TEXT NOT NULL DEFAULT '',
	damage_dice TEXT NOT NULL DEFAULT '',
	damage_type TEXT NOT NULL DEFAULT ''
);
CREATE INDEX character_spells_character ON character_spells(character_id);
CREATE UNIQUE INDEX character_spells_srd ON character_spells(character_id, srd_index, source_class) WHERE srd_index <> '';

CREATE TABLE character_spell_slots (
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	level INTEGER NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (character_id, level)
);

CREATE TABLE character_features (
	id INTEGER PRIMARY KEY,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	srd_index TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	class_index TEXT NOT NULL DEFAULT '',
	level INTEGER NOT NULL DEFAULT 0,
	description TEXT NOT NULL DEFAULT '',
	uses_max INTEGER NOT NULL DEFAULT 0,
	uses_used INTEGER NOT NULL DEFAULT 0,
	recharge TEXT NOT NULL DEFAULT ''
);
CREATE INDEX character_features_character ON character_features(character_id);
CREATE UNIQUE INDEX character_features_srd ON character_features(character_id, srd_index) WHERE srd_index <> '';

CREATE TABLE character_resources (
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	key TEXT NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (character_id, key)
);

CREATE TABLE character_rolls (
	id INTEGER PRIMARY KEY,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	expression TEXT NOT NULL DEFAULT '',
	detail TEXT NOT NULL DEFAULT '',
	total INTEGER NOT NULL DEFAULT 0,
	has_total INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL
);
CREATE INDEX character_rolls_character ON character_rolls(character_id, id);
`}
