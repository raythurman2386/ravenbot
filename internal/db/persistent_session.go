package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/session"
)

// SQLiteSessionService implements session.Service using SQLite.
type SQLiteSessionService struct {
	db *DB
}

func NewSQLiteSessionService(db *DB) *SQLiteSessionService {
	return &SQLiteSessionService{db: db}
}

func (s *SQLiteSessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	stateJSON, err := json.Marshal(req.State)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	query := `INSERT INTO sessions (app_name, user_id, session_id, state) VALUES (?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, req.AppName, req.UserID, sessionID, string(stateJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Persist initial state if any
	for k, v := range req.State {
		if err := s.persistState(ctx, req.AppName, req.UserID, k, v); err != nil {
			slog.Error("Failed to persist initial state", "key", k, "error", err)
		}
	}

	resp, err := s.Get(ctx, &session.GetRequest{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}

	return &session.CreateResponse{
		Session: resp.Session,
	}, nil
}

func (s *SQLiteSessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	var stateJSON string
	var updatedAt time.Time
	query := `SELECT state, updated_at FROM sessions WHERE app_name = ? AND user_id = ? AND session_id = ?`
	err := s.db.QueryRowContext(ctx, query, req.AppName, req.UserID, req.SessionID).Scan(&stateJSON, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", req.SessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var stateMap map[string]any
	if err := json.Unmarshal([]byte(stateJSON), &stateMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
	}

	// Load scoped states
	mergedState := make(map[string]any)

	// 1. App State
	appState, err := s.loadScopedState(ctx, "app", req.AppName, "")
	if err == nil {
		for k, v := range appState {
			mergedState["app:"+k] = v
		}
	}

	// 2. User State
	userState, err := s.loadScopedState(ctx, "user", req.AppName, req.UserID)
	if err == nil {
		for k, v := range userState {
			mergedState["user:"+k] = v
		}
	}

	// 3. Session State
	for k, v := range stateMap {
		mergedState[k] = v
	}

	// Load events
	eventQuery := `SELECT event_json FROM session_events WHERE app_name = ? AND user_id = ? AND session_id = ? ORDER BY timestamp ASC`
	if !req.After.IsZero() {
		eventQuery = `SELECT event_json FROM session_events WHERE app_name = ? AND user_id = ? AND session_id = ? AND timestamp >= ? ORDER BY timestamp ASC`
	}

	var rows *sql.Rows
	if req.After.IsZero() {
		rows, err = s.db.QueryContext(ctx, eventQuery, req.AppName, req.UserID, req.SessionID)
	} else {
		rows, err = s.db.QueryContext(ctx, eventQuery, req.AppName, req.UserID, req.SessionID, req.After)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}
	defer rows.Close()

	var events []*session.Event
	for rows.Next() {
		var ej string
		if err := rows.Scan(&ej); err != nil {
			continue
		}
		var e session.Event
		if err := json.Unmarshal([]byte(ej), &e); err == nil {
			events = append(events, &e)
		}
	}

	slog.Info("Loaded session events", "sessionID", req.SessionID, "count", len(events))

	if req.NumRecentEvents > 0 && len(events) > req.NumRecentEvents {
		events = events[len(events)-req.NumRecentEvents:]
	}

	ps := &persistentSession{
		svc:       s,
		appName:   req.AppName,
		userID:    req.UserID,
		sessionID: req.SessionID,
		state:     mergedState,
		events:    events,
		updatedAt: updatedAt,
	}

	return &session.GetResponse{Session: ps}, nil
}

func (s *SQLiteSessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	query := `SELECT session_id FROM sessions WHERE app_name = ? AND user_id = ?`
	rows, err := s.db.QueryContext(ctx, query, req.AppName, req.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			continue
		}
		resp, err := s.Get(ctx, &session.GetRequest{
			AppName:   req.AppName,
			UserID:    req.UserID,
			SessionID: sid,
		})
		if err == nil {
			sessions = append(sessions, resp.Session)
		}
	}
	return &session.ListResponse{Sessions: sessions}, nil
}

func (s *SQLiteSessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE app_name = ? AND user_id = ? AND session_id = ?`, req.AppName, req.UserID, req.SessionID)
	return err
}

func (s *SQLiteSessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	if event.Partial {
		return nil
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 1. Insert event
	_, err = tx.ExecContext(ctx, `INSERT INTO session_events (id, app_name, user_id, session_id, event_json, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		event.ID, sess.AppName(), sess.UserID(), sess.ID(), string(eventJSON), event.Timestamp)
	if err != nil {
		return err
	}

	// 2. Update session timestamp
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE app_name = ? AND user_id = ? AND session_id = ?`,
		event.Timestamp, sess.AppName(), sess.UserID(), sess.ID())
	if err != nil {
		return err
	}

	// 3. Update state if any
	if len(event.Actions.StateDelta) > 0 {
		// We need to merge session-specific delta into the sessions table
		// and scoped deltas into persistent_states
		var currentSessionStateJSON string
		err = tx.QueryRowContext(ctx, `SELECT state FROM sessions WHERE app_name = ? AND user_id = ? AND session_id = ?`,
			sess.AppName(), sess.UserID(), sess.ID()).Scan(&currentSessionStateJSON)
		if err != nil {
			return err
		}

		var sessionState map[string]any
		if err := json.Unmarshal([]byte(currentSessionStateJSON), &sessionState); err != nil {
			return fmt.Errorf("failed to unmarshal session state: %w", err)
		}
		if sessionState == nil {
			sessionState = make(map[string]any)
		}

		for k, v := range event.Actions.StateDelta {
			if len(k) > 4 && k[:4] == "app:" {
				slog.Debug("Persisting app state", "key", k[4:])
				if err := s.persistStateTx(ctx, tx, "app", sess.AppName(), "", k[4:], v); err != nil {
					return err
				}
			} else if len(k) > 5 && k[:5] == "user:" {
				slog.Debug("Persisting user state", "key", k[5:])
				if err := s.persistStateTx(ctx, tx, "user", sess.AppName(), sess.UserID(), k[5:], v); err != nil {
					return err
				}
			} else if len(k) > 5 && k[:5] == "temp:" {
				continue // Skip temp state
			} else {
				sessionState[k] = v
			}
		}

		newStateJSON, _ := json.Marshal(sessionState)
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET state = ? WHERE app_name = ? AND user_id = ? AND session_id = ?`,
			string(newStateJSON), sess.AppName(), sess.UserID(), sess.ID())
		if err != nil {
			return err
		}
	}

	// 4. Update the in-memory session object if applicable
	// This ensures that the current runner/agent sees the new event and state
	// updates immediately without needing to call Get() again.
	if ps, ok := sess.(*persistentSession); ok {
		ps.mu.Lock()
		ps.events = append(ps.events, event)
		ps.updatedAt = event.Timestamp
		// If we updated session state in the DB, also update it here
		if len(event.Actions.StateDelta) > 0 {
			for k, v := range event.Actions.StateDelta {
				// Only update session-local state (non-scoped)
				if (len(k) <= 4 || k[:4] != "app:") && (len(k) <= 5 || k[:5] != "user:") && (len(k) <= 5 || k[:5] != "temp:") {
					ps.state[k] = v
				}
			}
		}
		ps.mu.Unlock()
	}

	return tx.Commit()
}

func (s *SQLiteSessionService) persistState(ctx context.Context, appName, userID, key string, value any) error {
	scope := "session"
	actualKey := key
	if len(key) > 4 && key[:4] == "app:" {
		scope = "app"
		actualKey = key[4:]
		userID = ""
	} else if len(key) > 5 && key[:5] == "user:" {
		scope = "user"
		actualKey = key[5:]
	} else if len(key) > 5 && key[:5] == "temp:" {
		return nil
	}

	if scope == "session" {
		return nil // Handled in Create/AppendEvent
	}

	valJSON, _ := json.Marshal(value)
	query := `INSERT INTO persistent_states (scope, app_name, user_id, key, value_json) VALUES (?, ?, ?, ?, ?)
	          ON CONFLICT(scope, app_name, user_id, key) DO UPDATE SET value_json = excluded.value_json`
	_, err := s.db.ExecContext(ctx, query, scope, appName, userID, actualKey, string(valJSON))
	return err
}

func (s *SQLiteSessionService) persistStateTx(ctx context.Context, tx *sql.Tx, scope, appName, userID, key string, value any) error {
	valJSON, _ := json.Marshal(value)
	query := `INSERT INTO persistent_states (scope, app_name, user_id, key, value_json) VALUES (?, ?, ?, ?, ?)
	          ON CONFLICT(scope, app_name, user_id, key) DO UPDATE SET value_json = excluded.value_json`
	_, err := tx.ExecContext(ctx, query, scope, appName, userID, key, string(valJSON))
	return err
}

func (s *SQLiteSessionService) loadScopedState(ctx context.Context, scope, appName, userID string) (map[string]any, error) {
	query := `SELECT key, value_json FROM persistent_states WHERE scope = ? AND app_name = ? AND user_id = ?`
	rows, err := s.db.QueryContext(ctx, query, scope, appName, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]any)
	for rows.Next() {
		var k, vj string
		if err := rows.Scan(&k, &vj); err != nil {
			continue
		}
		var val any
		if err := json.Unmarshal([]byte(vj), &val); err == nil {
			res[k] = val
		}
	}
	return res, nil
}

// persistentSession implements session.Session.
type persistentSession struct {
	svc       *SQLiteSessionService
	appName   string
	userID    string
	sessionID string
	state     map[string]any
	events    []*session.Event
	updatedAt time.Time
	mu        sync.RWMutex
}

func (p *persistentSession) ID() string                { return p.sessionID }
func (p *persistentSession) AppName() string           { return p.appName }
func (p *persistentSession) UserID() string            { return p.userID }
func (p *persistentSession) LastUpdateTime() time.Time { return p.updatedAt }

func (p *persistentSession) State() session.State {
	return &persistentState{ps: p}
}

func (p *persistentSession) Events() session.Events {
	return persistentEvents(p.events)
}

// persistentState implements session.State.
type persistentState struct {
	ps *persistentSession
}

func (ps *persistentState) Get(key string) (any, error) {
	ps.ps.mu.RLock()
	defer ps.ps.mu.RUnlock()
	val, ok := ps.ps.state[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return val, nil
}

func (ps *persistentState) Set(key string, value any) error {
	ps.ps.mu.Lock()
	ps.ps.state[key] = value
	ps.ps.mu.Unlock()

	// Note: Set() on session.State interface in ADK usually also persists to storage
	// If it's a scoped key, we persist accordingly.
	return ps.ps.svc.persistState(context.Background(), ps.ps.AppName(), ps.ps.UserID(), key, value)
}

func (ps *persistentState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		ps.ps.mu.RLock()
		defer ps.ps.mu.RUnlock()
		for k, v := range ps.ps.state {
			if !yield(k, v) {
				return
			}
		}
	}
}

// persistentEvents implements session.Events.
type persistentEvents []*session.Event

func (e persistentEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, ev := range e {
			if !yield(ev) {
				return
			}
		}
	}
}

func (e persistentEvents) Len() int { return len(e) }
func (e persistentEvents) At(i int) *session.Event {
	if i < 0 || i >= len(e) {
		return nil
	}
	return e[i]
}
