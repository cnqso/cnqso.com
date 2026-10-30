package geo

import (
	"os"
	"testing"
)

func TestLookupWithoutDatabaseOrForPrivateIPs(t *testing.T) {
	for _, ip := range []string{"", "not-an-ip", "10.0.0.1", "127.0.0.1", "::1"} {
		if got := Lookup(ip) + Country(ip); got != "" {
			t.Fatalf("Lookup(%q) = %q", ip, got)
		}
	}
}

func TestLookupWithDatabase(t *testing.T) {
	path := os.Getenv("CNQSO_TEST_GEO_DB")
	if path == "" {
		t.Skip("set CNQSO_TEST_GEO_DB to a DB-IP City Lite mmdb")
	}
	if err := open(path); err != nil {
		t.Fatal(err)
	}
	if got := Lookup("8.8.8.8"); got == "" || got[len(got)-2:] != "US" {
		t.Fatalf("Lookup(8.8.8.8) = %q", got)
	}
	if got := Country("2001:4860:4860::8888"); got == "" {
		t.Fatal("IPv6 has no country")
	}
	if got := Lookup("10.0.0.1"); got != "" {
		t.Fatalf("private IP got %q", got)
	}
}
