package logs

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"server/config"
	"strings"
	"time"
)

// A process-local key prevents hashes being compared across servers or restarts.
// Daily domain separation limits recognition without creating a persistent secret.
var fingerprintKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("cannot initialize analytics randomness")
	}
	return key
}()

// This is a low-entropy browser-settings signature, not proof of identity. Never
// include IPs, cookies, URLs, or authenticated identity in its inputs.
func passiveFingerprint(r *http.Request, at time.Time) string {
	if !config.Fingerprinting || privateAnalyticsPath(r.URL.Path) || r.Header.Get("Sec-GPC") == "1" || r.Header.Get("DNT") == "1" {
		return ""
	}
	userAgent := fingerprintHeader(r.UserAgent(), 1024)
	if userAgent == "" {
		return ""
	}
	signals := []string{
		"passive-v1", at.UTC().Format("2006-01-02"), strings.ToLower(r.Host), userAgent,
		fingerprintHeader(r.Header.Get("Accept-Language"), 512),
		fingerprintHeader(r.Header.Get("Sec-CH-UA"), 512),
		fingerprintHeader(r.Header.Get("Sec-CH-UA-Platform"), 128),
		fingerprintHeader(r.Header.Get("Sec-CH-UA-Mobile"), 16),
	}
	encoded, _ := json.Marshal(signals)
	digest := hmac.New(sha256.New, fingerprintKey)
	digest.Write(encoded)
	return hex.EncodeToString(digest.Sum(nil)[:16])
}

func fingerprintHeader(value string, limit int) string {
	if len(value) > limit {
		value = value[:limit]
	}
	return strings.Join(strings.Fields(value), " ")
}
