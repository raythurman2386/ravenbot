package tools

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set ALLOW_LOCAL_URLS=true to allow httptest servers during tests
	os.Setenv("ALLOW_LOCAL_URLS", "true")
	code := m.Run()
	os.Exit(code)
}
