package agent

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"sync"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// KeyManager manages a set of Gemini API keys and their rate-limiting status.
type KeyManager struct {
	keys         []string
	currentIndex int
	mu           sync.Mutex
}

func NewKeyManager(keys []string) *KeyManager {
	return &KeyManager{
		keys: keys,
	}
}

func (km *KeyManager) GetCurrentKey() string {
	km.mu.Lock()
	defer km.mu.Unlock()
	return km.keys[km.currentIndex]
}

func (km *KeyManager) Rotate() string {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.currentIndex = (km.currentIndex + 1) % len(km.keys)
	slog.Info("Rotating Gemini API key", "newIndex", km.currentIndex)
	return km.keys[km.currentIndex]
}

func (km *KeyManager) NumKeys() int {
	return len(km.keys)
}

// RotatingLLM wraps model.LLM to provide automatic key rotation on rate limits.
type RotatingLLM struct {
	km         *KeyManager
	modelName  string
	backend    genai.Backend
	ctx        context.Context // Original context used to recreate models
	mu         sync.RWMutex
	currentLLM model.LLM
}

func NewRotatingLLM(ctx context.Context, km *KeyManager, modelName string, backend genai.Backend) (*RotatingLLM, error) {
	r := &RotatingLLM{
		km:        km,
		modelName: modelName,
		backend:   backend,
		ctx:       ctx,
	}
	if err := r.initLLM(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingLLM) initLLM(ctx context.Context) error {
	key := r.km.GetCurrentKey()
	llm, err := gemini.NewModel(ctx, r.modelName, &genai.ClientConfig{
		APIKey:  key,
		Backend: r.backend,
	})
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.currentLLM = llm
	r.mu.Unlock()
	return nil
}

func (r *RotatingLLM) Name() string {
	return r.modelName
}

func (r *RotatingLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, streaming bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for attempt := 0; attempt < r.km.NumKeys(); attempt++ {
			r.mu.RLock()
			llm := r.currentLLM
			r.mu.RUnlock()

			respIter := llm.GenerateContent(ctx, req, streaming)

			// We need to capture the first error to check for rate limiting
			var firstErr error
			success := false

			for resp, err := range respIter {
				if err != nil {
					firstErr = err
					break
				}
				success = true
				if !yield(resp, nil) {
					return
				}
			}

			if firstErr == nil {
				return // Success
			}

			// Check if it's a rate limit error (429)
			errMsg := firstErr.Error()
			if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "Resource has been exhausted") || strings.Contains(errMsg, "rate limit") {
				slog.Warn("Rate limit hit, attempting key rotation", "attempt", attempt+1, "error", errMsg)
				r.km.Rotate()
				if err := r.initLLM(r.ctx); err != nil {
					slog.Error("Failed to re-initialize LLM after rotation", "error", err)
					yield(nil, fmt.Errorf("failed to rotate key: %w", err))
					return
				}
				// If we already yielded some parts (in streaming mode), retrying might be tricky.
				// However, for most agentic flows, a 429 usually happens at the start.
				// If success was true, we can't easily restart the generator without duplicating output.
				if success && streaming {
					slog.Error("Rate limit hit mid-stream, cannot transparently retry")
					yield(nil, firstErr)
					return
				}
				continue // Retry with next key
			}

			// Not a rate limit error, propagate it
			yield(nil, firstErr)
			return
		}

		yield(nil, fmt.Errorf("exhausted all API keys and still hitting rate limits"))
	}
}
