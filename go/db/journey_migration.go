package db

import "database/sql"

// New fields are nullable so existing history remains untouched.
func migrateJourneys(handle *sql.DB) error {
	rows, err := handle.Query("PRAGMA table_info(access_logs)")
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var id, required, pk int
		var name, kind string
		var fallback any
		if err = rows.Scan(&id, &name, &kind, &required, &fallback, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	tx, err := handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, name := range []string{"visitor_id", "event_kind", "referrer", "fingerprint"} {
		if !columns[name] {
			if _, err = tx.Exec("ALTER TABLE access_logs ADD COLUMN " + name + " TEXT"); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS access_logs_visitor_time ON access_logs(visitor_id,timestamp,id);
 CREATE INDEX IF NOT EXISTS access_logs_journey_time ON access_logs(timestamp,id) WHERE event_kind IN ('page','download');
 CREATE INDEX IF NOT EXISTS access_logs_fingerprint_time ON access_logs(fingerprint,timestamp,visitor_id) WHERE fingerprint IS NOT NULL AND fingerprint!='';`); err != nil {
		return err
	}
	return tx.Commit()
}
