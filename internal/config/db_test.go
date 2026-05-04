package config

import (
	"strings"
	"testing"
)

func TestPostgresURL_escapesSpecialCharsInPassword(t *testing.T) {
	c := Config{
		DBHost:     "postgres",
		DBPort:     "5432",
		DBUser:     "u",
		DBPassword: "p@ss:word/ok?",
		DBName:     "shopping",
		DBSSLMode:  "disable",
	}
	u := c.PostgresURL()
	if !strings.Contains(u, "@postgres:5432") {
		t.Fatalf("expected host in url, got %q", u)
	}
	if strings.Contains(u, "p@ss") {
		t.Fatalf("raw password must not appear in url: %q", u)
	}
}
