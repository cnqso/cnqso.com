package db

import (
	"database/sql"
	"net"
	"net/netip"
	"strings"
)

// Preserve raw historical addresses for inspection while grouping on actual IPs.
// DDL and backfill commit together, so interruption cannot leave a partial upgrade.
func migrateClientIPs(handle *sql.DB) error {
	rows, err := handle.Query("PRAGMA table_info(access_logs)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var id, notNull, pk int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&id, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "client_ip" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if found {
		var pending bool
		if err = handle.QueryRow("SELECT EXISTS(SELECT 1 FROM access_logs WHERE client_ip IS NULL)").Scan(&pending); err != nil {
			return err
		}
		if !pending {
			return nil
		}
	}
	tx, err := handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if !found {
		if _, err = tx.Exec("ALTER TABLE access_logs ADD COLUMN client_ip TEXT"); err != nil {
			return err
		}
	}
	// The temporary index bounds work per distinct old address during the backfill.
	if _, err = tx.Exec("CREATE INDEX access_logs_ip_backfill ON access_logs(remote_addr)"); err != nil {
		return err
	}
	rows, err = tx.Query("SELECT DISTINCT remote_addr FROM access_logs WHERE client_ip IS NULL")
	if err != nil {
		return err
	}
	var addresses []sql.NullString
	for rows.Next() {
		var address sql.NullString
		if err = rows.Scan(&address); err != nil {
			rows.Close()
			return err
		}
		addresses = append(addresses, address)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	update, err := tx.Prepare("UPDATE access_logs SET client_ip=? WHERE remote_addr IS ? AND client_ip IS NULL")
	if err != nil {
		return err
	}
	defer update.Close()
	for _, address := range addresses {
		if _, err = update.Exec(historicalClientIP(address.String), address); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DROP INDEX access_logs_ip_backfill"); err != nil {
		return err
	}
	return tx.Commit()
}

func historicalClientIP(raw string) string {
	// Nginx appended its observed client address to historical forwarded chains.
	if i := strings.LastIndexByte(raw, ','); i >= 0 {
		raw = raw[i+1:]
	}
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	if ip, err := netip.ParseAddr(raw); err == nil {
		return ip.Unmap().String()
	}
	return "unknown"
}
