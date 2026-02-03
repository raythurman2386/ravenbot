package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyManagerRotation(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys)

	assert.Equal(t, "key1", km.GetCurrentKey())

	km.Rotate()
	assert.Equal(t, "key2", km.GetCurrentKey())

	km.Rotate()
	assert.Equal(t, "key3", km.GetCurrentKey())

	km.Rotate()
	assert.Equal(t, "key1", km.GetCurrentKey())
}
