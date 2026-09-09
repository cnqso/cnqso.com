package db

import (
	"database/sql"
	"server/internal/requestlog"
)

// Recheck on startup to also clean records written by a temporarily rolled-back
// server. Keep event counts and every non-URL field; only remove unsafe URL data.
func sanitizeHistoricalURLs(handle *sql.DB) error {
	tx, err := handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Overwrite removed data in SQLite pages, not just their logical values.
	if _, err = tx.Exec("PRAGMA secure_delete=ON"); err != nil {
		return err
	}
	rows, err := tx.Query("SELECT id, url FROM access_logs WHERE url IS NOT NULL")
	if err != nil {
		return err
	}
	type change struct {
		id  int64
		url string
	}
	var changes []change
	for rows.Next() {
		var id int64
		var raw string
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		if clean := requestlog.Path(raw); clean != raw {
			changes = append(changes, change{id, clean})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(changes) != 0 {
		update, err := tx.Prepare("UPDATE access_logs SET url=? WHERE id=?")
		if err != nil {
			return err
		}
		defer update.Close()
		for _, c := range changes {
			if _, err = update.Exec(c.url, c.id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
