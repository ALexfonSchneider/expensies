package postgresrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	settingReceiptSession = "lkdr_session"
	settingReceiptSync    = "lkdr_sync"
)

// sessionJSON mirrors domain.ReceiptSession with JSON tags so the domain
// type stays free of serialization concerns.
type sessionJSON struct {
	Phone            string    `json:"phone"`
	DeviceID         string    `json:"device_id"`
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenExpiresAt   time.Time `json:"token_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type syncJSON struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Listed     int       `json:"listed"`
	Added      int       `json:"added"`
	Detailed   int       `json:"detailed"`
	Pending    int       `json:"pending"`
	Matched    int       `json:"matched"`
	Error      string    `json:"error"`
}

func (r *Settings) load(ctx context.Context, key string, v any) (bool, error) {
	var raw string
	err := r.q().QueryRow(ctx, `SELECT value::text FROM settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("postgresrepo: load setting %s: %w", key, err)
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return false, fmt.Errorf("postgresrepo: decode setting %s: %w", key, err)
	}
	return true, nil
}

func (r *Settings) save(ctx context.Context, key string, v any, now time.Time) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("postgresrepo: encode setting %s: %w", key, err)
	}
	_, err = r.q().Exec(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2::jsonb, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		key, string(raw), now)
	if err != nil {
		return fmt.Errorf("postgresrepo: save setting %s: %w", key, err)
	}
	return nil
}

// Load implements domain.ReceiptSessionStore.
func (r *Settings) Load(ctx context.Context) (*domain.ReceiptSession, error) {
	var s sessionJSON
	ok, err := r.load(ctx, settingReceiptSession, &s)
	if err != nil || !ok {
		return nil, err
	}
	return &domain.ReceiptSession{
		Phone:            s.Phone,
		DeviceID:         s.DeviceID,
		AccessToken:      s.AccessToken,
		RefreshToken:     s.RefreshToken,
		TokenExpiresAt:   s.TokenExpiresAt,
		RefreshExpiresAt: s.RefreshExpiresAt,
		UpdatedAt:        s.UpdatedAt,
	}, nil
}

// Save implements domain.ReceiptSessionStore.
func (r *Settings) Save(ctx context.Context, s *domain.ReceiptSession) error {
	return r.save(ctx, settingReceiptSession, sessionJSON{
		Phone:            s.Phone,
		DeviceID:         s.DeviceID,
		AccessToken:      s.AccessToken,
		RefreshToken:     s.RefreshToken,
		TokenExpiresAt:   s.TokenExpiresAt,
		RefreshExpiresAt: s.RefreshExpiresAt,
		UpdatedAt:        s.UpdatedAt,
	}, s.UpdatedAt)
}

// Clear implements domain.ReceiptSessionStore.
func (r *Settings) Clear(ctx context.Context) error {
	if _, err := r.q().Exec(ctx, `DELETE FROM settings WHERE key = $1`, settingReceiptSession); err != nil {
		return fmt.Errorf("postgresrepo: clear session: %w", err)
	}
	return nil
}

// LoadSyncStatus implements domain.ReceiptSyncStore.
func (r *Settings) LoadSyncStatus(ctx context.Context) (*domain.ReceiptSyncStatus, error) {
	var s syncJSON
	ok, err := r.load(ctx, settingReceiptSync, &s)
	if err != nil || !ok {
		return nil, err
	}
	return &domain.ReceiptSyncStatus{
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Listed:     s.Listed,
		Added:      s.Added,
		Detailed:   s.Detailed,
		Pending:    s.Pending,
		Matched:    s.Matched,
		Error:      s.Error,
	}, nil
}

// SaveSyncStatus implements domain.ReceiptSyncStore.
func (r *Settings) SaveSyncStatus(ctx context.Context, st *domain.ReceiptSyncStatus, now time.Time) error {
	return r.save(ctx, settingReceiptSync, syncJSON{
		StartedAt:  st.StartedAt,
		FinishedAt: st.FinishedAt,
		Listed:     st.Listed,
		Added:      st.Added,
		Detailed:   st.Detailed,
		Pending:    st.Pending,
		Matched:    st.Matched,
		Error:      st.Error,
	}, now)
}
