package ollama

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set ALLOW_LOCAL_URLS=true to allow httptest servers during tests
	_ = os.Setenv("ALLOW_LOCAL_URLS", "true")
	code := m.Run()
	os.Exit(code)
}
