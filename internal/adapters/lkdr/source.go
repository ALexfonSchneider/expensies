// Package lkdr pulls receipts from the tax service archive "Мои чеки
// онлайн" through the community client github.com/jfk9w-go/lkdr-api.
//
// The interactive login (captcha plus SMS) is never performed here: the
// user signs in with a browser and hands over the refresh token, and the
// client only refreshes it. When even the refresh token is rejected the
// adapter reports domain.ErrReceiptSession so the user signs in again.
package lkdr

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jfk9w-go/based"
	lkdrapi "github.com/jfk9w-go/lkdr-api"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	// SourceName is stored with every receipt pulled from this archive.
	SourceName = "lkdr"

	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	orderNewestFirst = "RECEIVE_DATE:DESC"

	// Outside Russia the archive accepts the connection and never answers;
	// the library's HTTP client has no timeout of its own, so every call is
	// bounded here. The bound leaves room for the client's own
	// 20-per-minute throttle.
	callTimeout           = 90 * time.Second
	dialTimeout           = 10 * time.Second
	responseHeaderTimeout = 45 * time.Second
)

func defaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
		TLSHandshakeTimeout:   dialTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       60 * time.Second,
	}
}

// Source implements domain.ReceiptSource.
type Source struct {
	store       domain.ReceiptSessionStore
	userAgent   string
	now         func() time.Time
	transport   http.RoundTripper
	callTimeout time.Duration

	// The client caches tokens in memory, so it is rebuilt whenever the
	// stored session changes (a freshly pasted refresh token, a phone
	// switch), which sessionStamp tracks.
	mu           sync.Mutex
	client       *lkdrapi.Client
	sessionStamp string
}

// Option configures a Source.
type Option func(*Source)

// WithUserAgent overrides the browser signature sent to the archive.
func WithUserAgent(ua string) Option {
	return func(s *Source) {
		if ua != "" {
			s.userAgent = ua
		}
	}
}

// WithClock replaces the wall clock (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Source) {
		if now != nil {
			s.now = now
		}
	}
}

// WithTransport replaces the HTTP transport (tests).
func WithTransport(rt http.RoundTripper) Option {
	return func(s *Source) {
		if rt != nil {
			s.transport = rt
		}
	}
}

// WithCallTimeout bounds one archive request (tests).
func WithCallTimeout(d time.Duration) Option {
	return func(s *Source) {
		if d > 0 {
			s.callTimeout = d
		}
	}
}

// New creates a Source that reads its credentials from store.
func New(store domain.ReceiptSessionStore, opts ...Option) (*Source, error) {
	if store == nil {
		return nil, fmt.Errorf("lkdr: session store is required")
	}
	s := &Source{
		store:       store,
		userAgent:   defaultUserAgent,
		now:         time.Now,
		transport:   defaultTransport(),
		callTimeout: callTimeout,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// noLogin refuses the interactive part of the flow so the client can never
// start a captcha or SMS challenge on its own.
type noLogin struct{}

func (noLogin) GetCaptchaToken(context.Context, string, string, string) (string, error) {
	return "", domain.ErrReceiptSession
}

func (noLogin) GetConfirmationCode(context.Context, string) (string, error) {
	return "", domain.ErrReceiptSession
}

// tokenStorage adapts the session store to the client's token persistence.
type tokenStorage struct {
	store domain.ReceiptSessionStore
	now   func() time.Time
}

func (t tokenStorage) LoadTokens(ctx context.Context, phone string) (*lkdrapi.Tokens, error) {
	s, err := t.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if s == nil || s.Phone != phone || s.RefreshToken == "" {
		return nil, nil
	}
	tokens := &lkdrapi.Tokens{
		Token:         s.AccessToken,
		RefreshToken:  s.RefreshToken,
		TokenExpireIn: lkdrapi.DateTimeTZ(s.TokenExpiresAt),
	}
	if !s.RefreshExpiresAt.IsZero() {
		exp := lkdrapi.DateTimeTZ(s.RefreshExpiresAt)
		tokens.RefreshTokenExpiresIn = &exp
	}
	return tokens, nil
}

func (t tokenStorage) UpdateTokens(ctx context.Context, phone string, tokens *lkdrapi.Tokens) error {
	if tokens == nil {
		return t.store.Clear(ctx)
	}
	s, err := t.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if s == nil {
		s = &domain.ReceiptSession{Phone: phone}
	}
	s.AccessToken = tokens.Token
	s.RefreshToken = tokens.RefreshToken
	s.TokenExpiresAt = tokens.TokenExpireIn.Time()
	s.RefreshExpiresAt = time.Time{}
	if tokens.RefreshTokenExpiresIn != nil {
		s.RefreshExpiresAt = tokens.RefreshTokenExpiresIn.Time()
	}
	s.UpdatedAt = t.now()
	return t.store.Save(ctx, s)
}

func (s *Source) clientFor(ctx context.Context) (*lkdrapi.Client, context.Context, error) {
	session, err := s.store.Load(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("lkdr: load session: %w", err)
	}
	if session == nil || session.RefreshToken == "" {
		return nil, nil, domain.ErrReceiptSession
	}
	stamp := session.Phone + "|" + session.DeviceID + "|" + session.UpdatedAt.UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil || s.sessionStamp != stamp {
		c, err := lkdrapi.NewClient(lkdrapi.ClientParams{
			Phone:        session.Phone,
			Clock:        based.ClockFunc(s.now),
			DeviceID:     session.DeviceID,
			UserAgent:    s.userAgent,
			TokenStorage: tokenStorage{store: s.store, now: s.now},
			Transport:    s.transport,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("lkdr: create client: %w", err)
		}
		s.client = c
		s.sessionStamp = stamp
	}
	return s.client, lkdrapi.WithAuthorizer(ctx, noLogin{}), nil
}

// call runs one archive request under the call timeout.
func call[T any](ctx context.Context, timeout time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := fn(ctx)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		var zero T
		return zero, fmt.Errorf("archive did not answer within %s: %w", timeout, err)
	}
	return out, err
}

// ListReceipts implements domain.ReceiptSource.
func (s *Source) ListReceipts(ctx context.Context, since *time.Time, offset, limit int) ([]domain.Receipt, bool, error) {
	c, ctx, err := s.clientFor(ctx)
	if err != nil {
		return nil, false, err
	}
	in := &lkdrapi.ReceiptIn{Limit: limit, Offset: offset, OrderBy: orderNewestFirst}
	if since != nil {
		d := lkdrapi.Date(*since)
		in.DateFrom = &d
	}
	out, err := call(ctx, s.callTimeout, func(ctx context.Context) (*lkdrapi.ReceiptOut, error) {
		return c.Receipt(ctx, in)
	})
	if err != nil {
		return nil, false, wrapErr("list receipts", err)
	}
	list := make([]domain.Receipt, 0, len(out.Receipts))
	for _, r := range out.Receipts {
		total, err := parseMoney(r.TotalSum)
		if err != nil {
			return nil, false, fmt.Errorf("lkdr: receipt %s: total %q: %w", r.Key, r.TotalSum, err)
		}
		list = append(list, domain.Receipt{
			Source:               SourceName,
			Key:                  r.Key,
			FiscalDriveNumber:    r.FiscalDriveNumber,
			FiscalDocumentNumber: r.FiscalDocumentNumber,
			SellerName:           strings.TrimSpace(r.KktOwner),
			SellerINN:            r.KktOwnerInn,
			IssuedAt:             r.CreatedDate.Time(),
			ReceivedAt:           r.ReceiveDate.Time(),
			Total:                total,
		})
	}
	return list, out.HasMore, nil
}

// LoadDetails implements domain.ReceiptSource.
func (s *Source) LoadDetails(ctx context.Context, r *domain.Receipt) error {
	c, ctx, err := s.clientFor(ctx)
	if err != nil {
		return err
	}
	out, err := call(ctx, s.callTimeout, func(ctx context.Context) (*lkdrapi.FiscalDataOut, error) {
		return c.FiscalData(ctx, &lkdrapi.FiscalDataIn{Key: r.Key})
	})
	if lkdrapi.IsDataNotFound(err) {
		return fmt.Errorf("lkdr: receipt %s: %w", r.Key, domain.ErrReceiptDetailsUnavailable)
	}
	if err != nil {
		return wrapErr("fiscal data", err)
	}

	scale := fiscalScale(out.TotalSum, r.Total)
	r.IssuedAt = out.DateTime.Time()
	if u := strings.TrimSpace(deref(out.User)); u != "" {
		r.SellerName = u
	}
	if out.UserInn != "" {
		r.SellerINN = out.UserInn
	}
	r.RetailPlace = strings.TrimSpace(deref(out.RetailPlace))
	r.RetailAddress = strings.TrimSpace(deref(out.RetailPlaceAddress))
	r.FiscalSign = out.FiscalSign
	if out.FiscalDriveNumber != "" {
		r.FiscalDriveNumber = out.FiscalDriveNumber
	}
	if r.FiscalDocumentNumber == "" && out.FiscalDocumentNumber != 0 {
		r.FiscalDocumentNumber = strconv.FormatInt(out.FiscalDocumentNumber, 10)
	}
	r.OperationType = out.OperationType
	r.Total = toMoney(out.TotalSum, scale)
	r.CashTotal = toMoney(out.CashTotalSum, scale)
	r.EcashTotal = toMoney(out.EcashTotalSum, scale)

	items := make([]domain.ReceiptItem, 0, len(out.Items))
	for i, it := range out.Items {
		items = append(items, domain.ReceiptItem{
			Position:    i + 1,
			Name:        strings.Join(strings.Fields(it.Name), " "),
			Price:       toMoney(it.Price, scale),
			Quantity:    it.Quantity,
			Sum:         toMoney(it.Sum, scale),
			ProductType: it.ProductType,
		})
	}
	r.Items = items
	r.ItemCount = len(items)
	r.ItemsLoaded = true
	r.ItemsError = ""
	return nil
}

// Probe implements domain.ReceiptSource.
func (s *Source) Probe(ctx context.Context) error {
	_, _, err := s.ListReceipts(ctx, nil, 0, 1)
	return err
}

// wrapErr turns authorization failures into domain.ErrReceiptSession so
// the UI can ask for a new sign-in instead of showing a raw HTTP error.
func wrapErr(op string, err error) error {
	if errors.Is(err, domain.ErrReceiptSession) {
		return fmt.Errorf("lkdr: %s: %w", op, err)
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"refresh token", "authorize", "401", "403", "unauthorized", "forbidden"} {
		if strings.Contains(msg, marker) {
			return fmt.Errorf("lkdr: %s: %w: %v", op, domain.ErrReceiptSession, err)
		}
	}
	return fmt.Errorf("lkdr: %s: %w", op, err)
}

// fiscalScale tells whether the fiscal document reports rubles or kopecks
// by comparing its total with the listing total, which is decimal rubles.
// The archive is not documented, so the unit is detected per receipt.
func fiscalScale(docTotal float64, listTotal domain.Money) float64 {
	if listTotal <= 0 || docTotal <= 0 {
		return 100
	}
	if math.Abs(docTotal-float64(listTotal)) < 0.5 {
		return 1
	}
	return 100
}

func toMoney(v, scale float64) domain.Money {
	return domain.Money(math.Round(v * scale))
}

// parseMoney reads decimal rubles ("1234.56", "1 234,5", "500") into
// kopecks without going through floating point.
func parseMoney(s string) (domain.Money, error) {
	s = strings.NewReplacer(" ", "", " ", "", ",", ".").Replace(strings.TrimSpace(s))
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	m := domain.Money(w*100 + f)
	if negative {
		m = -m
	}
	return m, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
