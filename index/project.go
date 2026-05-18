// Package index provides a cross-CLI project index that maps
// working directories to per-CLI native keys.
package index

import (
	"database/sql"
	"fmt"
	"time"

	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const schema = `CREATE TABLE IF NOT EXISTS projects (
	cwd        TEXT    NOT NULL,
	cli        TEXT    NOT NULL,
	native_key TEXT    NOT NULL,
	last_seen  INTEGER NOT NULL,
	PRIMARY KEY (cwd, cli)
);`

// ProjectIndex is a persistent mapping from cwd to per-CLI native
// project keys, backed by SQLite.
type ProjectIndex struct {
	db *sql.DB
}

// Open opens or creates the project index at the given path.
func Open(path string) (*ProjectIndex, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("index: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("index: migrate: %w", err)
	}
	return &ProjectIndex{db: db}, nil
}

// Register stores the per-CLI native keys for a project cwd.
// Existing entries for the same (cwd, cli) pair are overwritten.
func (idx *ProjectIndex) Register(
	cwd string, entries map[string]string,
) error {
	now := time.Now().Unix()
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("index: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO projects (cwd, cli, native_key, last_seen)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(cwd, cli)
		DO UPDATE SET native_key = excluded.native_key,
		              last_seen  = excluded.last_seen`)
	if err != nil {
		return fmt.Errorf("index: prepare: %w", err)
	}
	defer stmt.Close()

	for cli, key := range entries {
		if _, err := stmt.Exec(cwd, cli, key, now); err != nil {
			return fmt.Errorf("index: insert (%s, %s): %w", cwd, cli, err)
		}
	}
	return tx.Commit()
}

// Lookup returns all known CLI native keys for a cwd.
// Returns an empty (non-nil) map when the cwd is not registered.
func (idx *ProjectIndex) Lookup(cwd string) (map[string]string, error) {
	rows, err := idx.db.Query(
		`SELECT cli, native_key FROM projects WHERE cwd = ?`, cwd,
	)
	if err != nil {
		return nil, fmt.Errorf("index: lookup %s: %w", cwd, err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var cli, key string
		if err := rows.Scan(&cli, &key); err != nil {
			return nil, fmt.Errorf("index: scan: %w", err)
		}
		out[cli] = key
	}
	return out, rows.Err()
}

// Scan walks all registered adapters and builds/refreshes the index
// by calling ProjectKey on every cwd already known to each adapter.
func (idx *ProjectIndex) Scan(
	adapters []session.SessionAdapter,
) error {
	// Collect all known cwds from the index.
	rows, err := idx.db.Query(`SELECT DISTINCT cwd FROM projects`)
	if err != nil {
		return fmt.Errorf("index: scan cwds: %w", err)
	}
	defer rows.Close()

	var cwds []string
	for rows.Next() {
		var cwd string
		if err := rows.Scan(&cwd); err != nil {
			return fmt.Errorf("index: scan row: %w", err)
		}
		cwds = append(cwds, cwd)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// For each cwd, register keys from all adapters.
	for _, cwd := range cwds {
		entries := make(map[string]string, len(adapters))
		for _, a := range adapters {
			entries[string(a.CLI())] = a.ProjectKey(cwd)
		}
		if err := idx.Register(cwd, entries); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the underlying database.
func (idx *ProjectIndex) Close() error {
	return idx.db.Close()
}
