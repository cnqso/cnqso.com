package api

import (
	"context"
	"server/db"
)

type fingerprintMatch struct {
	ID, Label string
	Shared    int
}

// Compare only recently observed signatures. Cookie identities remain separate,
// even when multiple browsers report exactly the same fingerprint inputs.
func journeyFingerprintMatches(ctx context.Context, visitor string) ([]string, []fingerprintMatch, error) {
	rows, err := db.DB.QueryContext(ctx, `SELECT fingerprint FROM access_logs WHERE visitor_id=? AND timestamp>=datetime('now','-1 day') AND fingerprint IS NOT NULL AND fingerprint!='' GROUP BY fingerprint ORDER BY MAX(timestamp) DESC LIMIT 8`, visitor)
	if err != nil {
		return nil, nil, err
	}
	var signatures []string
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			rows.Close()
			return nil, nil, err
		}
		signatures = append(signatures, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if len(signatures) == 0 {
		return signatures, nil, nil
	}
	// The inner query is capped, and both sides use fingerprint/visitor indexes.
	rows, err = db.DB.QueryContext(ctx, `SELECT visitor_id,COUNT(DISTINCT fingerprint) FROM access_logs
 WHERE fingerprint IN (SELECT fingerprint FROM access_logs WHERE visitor_id=? AND timestamp>=datetime('now','-1 day') AND fingerprint IS NOT NULL AND fingerprint!='' GROUP BY fingerprint ORDER BY MAX(timestamp) DESC LIMIT 8)
 AND fingerprint IS NOT NULL AND fingerprint!='' AND timestamp>=datetime('now','-1 day') AND visitor_id!=? AND visitor_id IS NOT NULL AND visitor_id!=''
 GROUP BY visitor_id ORDER BY MAX(timestamp) DESC LIMIT 20`, visitor, visitor)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var matches []fingerprintMatch
	for rows.Next() {
		var match fingerprintMatch
		if err = rows.Scan(&match.ID, &match.Shared); err != nil {
			return nil, nil, err
		}
		if !validVisitorID(match.ID) {
			continue
		}
		match.Label = "Browser " + match.ID[:8]
		matches = append(matches, match)
	}
	return signatures, matches, rows.Err()
}
