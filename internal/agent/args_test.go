package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchRSSArgs_Unmarshal(t *testing.T) {
	// Object
	jsonObj := `{"url": "http://example.com"}`
	var argsObj FetchRSSArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonObj), &argsObj))
	assert.Equal(t, "http://example.com", argsObj.URL)

	// Array
	jsonArr := `["http://example.com/feed"]`
	var argsArr FetchRSSArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonArr), &argsArr))
	assert.Equal(t, "http://example.com/feed", argsArr.URL)
}

func TestWebSearchArgs_Unmarshal(t *testing.T) {
	// Object
	jsonObj := `{"query": "golang", "max_results": 10}`
	var argsObj WebSearchArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonObj), &argsObj))
	assert.Equal(t, "golang", argsObj.Query)
	assert.Equal(t, 10, argsObj.MaxResults)

	// Array [string]
	jsonArr1 := `["golang"]`
	var argsArr1 WebSearchArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonArr1), &argsArr1))
	assert.Equal(t, "golang", argsArr1.Query)
	assert.Equal(t, 0, argsArr1.MaxResults) // default

	// Array [string, int]
	jsonArr2 := `["python", 5]`
	var argsArr2 WebSearchArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonArr2), &argsArr2))
	assert.Equal(t, "python", argsArr2.Query)
	assert.Equal(t, 5, argsArr2.MaxResults)
}

func TestReadMCPResourceArgs_Unmarshal(t *testing.T) {
	// Object
	jsonObj := `{"server": "filesystem", "uri": "file:///tmp"}`
	var argsObj ReadMCPResourceArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonObj), &argsObj))
	assert.Equal(t, "filesystem", argsObj.Server)
	assert.Equal(t, "file:///tmp", argsObj.URI)

	// Array
	jsonArr := `["filesystem", "file:///tmp"]`
	var argsArr ReadMCPResourceArgs
	assert.NoError(t, json.Unmarshal([]byte(jsonArr), &argsArr))
	assert.Equal(t, "filesystem", argsArr.Server)
	assert.Equal(t, "file:///tmp", argsArr.URI)
}
