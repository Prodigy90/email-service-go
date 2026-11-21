package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/prodigy90/email-service-go/internal/db"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
)

// capTestDB opens EMAIL_SERVICE_TEST_DATABASE_URL (same contract as
// TestRecipientState_Integration: an empty DB gets migrations/, a prod
// `pg_dump -s` restore is used as is). Skipped when unset.
func capTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("EMAIL_SERVICE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("EMAIL_SERVICE_TEST_DATABASE_URL not set")
	}
	sqlxDB, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlxDB.Close() })
	var has bool
	if err := sqlxDB.Get(&has, `SELECT to_regclass('public.emails') IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if !has {
		if err := db.RunMigrations(sqlxDB.DB, "../../migrations"); err != nil {
			t.Fatal(err)
		}
	}
	return sqlxDB
}

type capWorld struct {
	t    *testing.T
	db   *sqlx.DB
	run  string
	svc  *EmailService
	sent []string // enqueued email ids
	mu   sync.Mutex
}

// newCapWorld builds the real EmailService (real repo, real templates) with
// the queue stubbed out, at a fixed clock.
func newCapWorld(t *testing.T, mode MarketingCapMode, now time.Time) *capWorld {
	t.Helper()
	sqlxDB := capTestDB(t)
	w := &capWorld{t: t, db: sqlxDB, run: uuid.NewString()[:8]}
	tmpl, err := NewTemplateService("../../pkg/templates", zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewEmailRepository(sqlxDB)
	w.svc = NewEmailService(repo, nil, tmpl, nil, nil, nil, zerolog.Nop())
	w.svc.now = func() time.Time { return now }
	w.svc.enqueueFn = func(e *domain.Email) error {
		w.mu.Lock()
		w.sent = append(w.sent, e.ID.String())
		w.mu.Unlock()
		return nil
	}
	w.svc.SetMarketingCap(MarketingCapConfig{Mode: mode, PerDay: 1})
	t.Cleanup(func() {
		sqlxDB.MustExec(`DELETE FROM emails WHERE lower(to_address) LIKE $1`, "%-"+w.run+"@cap-test.example")
	})
	return w
}

func (w *capWorld) addr(name string) string {
	return fmt.Sprintf("%s-%s@cap-test.example", name, w.run)
}

func (w *capWorld) send(to, template, source string, metadata map[string]interface{}, idem string) (*domain.SendEmailResponse, error) {
	req := &domain.SendEmailRequest{
		To: to, Template: template, SourceService: source, Metadata: metadata, IdempotencyID: idem,
		TemplateData: map[string]interface{}{"Name": "Ada", "Subject": "s", "Body": "b", "HTMLBody": "<p>b</p>"},
	}
	if template == "" {
		req.Subject, req.Body = "raw", "raw body"
	}
	return w.svc.Send(context.Background(), req)
}

// seed writes a row directly, as an earlier send would have left it.
func (w *capWorld) seed(to, status, template, source, metadata string, at time.Time) {
	var tmpl any
	if template != "" {
		tmpl = template
	}
	w.db.MustExec(`INSERT INTO emails (id, to_address, subject, body, template, status, source_service, metadata, created_at, updated_at)
		VALUES ($1, $2, 's', 'b', $3, $4, $5, $6::jsonb, $7, $7)`, uuid.New(), to, tmpl, status, source, metadata, at)
}

func (w *capWorld) rows(to string) int {
	var n int
	if err := w.db.Get(&n, `SELECT count(*) FROM emails WHERE lower(to_address) = lower($1)`, to); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func isCapped(err error) bool {
	var c *ErrMarketingCapped
	return errors.As(err, &c)
}

// 10:00 UTC on 21 Nov = 11:00 Lagos.
var capNow = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

func TestMarketingCap_Enforce_Integration(t *testing.T) {
	w := newCapWorld(t, CapEnforce, capNow)
	a := w.addr("ada")

	if _, err := w.send(a, "onboarding_day3", "wasbot", map[string]interface{}{"sequence": true}, ""); err != nil {
		t.Fatalf("first marketing send refused: %v", err)
	}
	_, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "vsl_reeng_d1pm"}, "cap-"+w.run+"-2")
	if !isCapped(err) {
		t.Fatalf("second marketing send: want ErrMarketingCapped, got %v", err)
	}
	var c *ErrMarketingCapped
	errors.As(err, &c)
	if c.MarketingToday != 1 || c.Cap != 1 || !c.RetryAfter.Equal(time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC)) {
		t.Errorf("refusal = %+v, want marketing_today 1, cap 1, retry_after next Lagos midnight", c)
	}
	if n := w.rows(a); n != 1 {
		t.Errorf("a refused send must write nothing: %d rows, want 1", n)
	}

	// Service mail always passes, however much marketing went out.
	for _, tmpl := range []string{"password_reset", "subscription_expiring_1d", "onboarding_day0", "checkout_abandoned", "commission_earned"} {
		if _, err := w.send(a, tmpl, "wasbot", nil, ""); err != nil {
			t.Errorf("service %s refused: %v", tmpl, err)
		}
	}
	// An untemplated support reply is not marketing; an untemplated campaign send is.
	if _, err := w.send(a, "", "support", nil, ""); err != nil {
		t.Errorf("untemplated support send refused: %v", err)
	}
	if _, err := w.send(a, "", "campaign", map[string]interface{}{"campaign": "x"}, ""); !isCapped(err) {
		t.Errorf("untemplated campaign send: want capped, got %v", err)
	}
	// Service sends after the first marketing one did not count toward it either.
	b := w.addr("bea")
	if _, err := w.send(b, "payment_success", "wasbot", nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.send(b, "onboarding_paid_day2", "wasbot", nil, ""); err != nil {
		t.Errorf("first marketing after a service send refused: %v", err)
	}
}

// A retry of an accepted send (same idempotency ID) gets its row back and is
// never refused by its own earlier copy.
func TestMarketingCap_IdempotentRetry_Integration(t *testing.T) {
	w := newCapWorld(t, CapEnforce, capNow)
	a := w.addr("retry")
	first, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "c"}, "cap-"+w.run+"-same")
	if err != nil {
		t.Fatal(err)
	}
	again, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "c"}, "cap-"+w.run+"-same")
	if err != nil {
		t.Fatalf("idempotent retry refused: %v", err)
	}
	if again.ID != first.ID || w.rows(a) != 1 {
		t.Errorf("retry should return the existing row: %s vs %s, rows %d", again.ID, first.ID, w.rows(a))
	}
}

// Concurrent marketing sends to one address: exactly one is accepted.
func TestMarketingCap_Race_Integration(t *testing.T) {
	w := newCapWorld(t, CapEnforce, capNow)
	a := w.addr("race")
	const n = 24
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, capped, other := 0, 0, 0
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			tmpl := "campaign_update"
			if i%2 == 0 {
				tmpl = "onboarding_day2"
			}
			_, err := w.send(a, tmpl, "campaign", map[string]interface{}{"campaign": "race"}, fmt.Sprintf("cap-%s-race-%d", w.run, i))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case isCapped(err):
				capped++
			default:
				other++
				t.Errorf("send %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if ok != 1 || capped != n-1 || other != 0 {
		t.Fatalf("concurrent sends: %d accepted, %d capped, %d errors; want 1/%d/0", ok, capped, other, n-1)
	}
	if r := w.rows(a); r != 1 {
		t.Fatalf("%d rows for the address, want exactly 1", r)
	}
}

// The day is the Lagos calendar day. At 00:45 Lagos on 22 Nov (23:45 UTC on
// the 21st), a send at 23:30 Lagos the night before (22:30 UTC) is
// yesterday; one at 00:30 Lagos (23:30 UTC) is today.
func TestMarketingCap_LagosDay_Integration(t *testing.T) {
	now := time.Date(2026, 11, 21, 23, 45, 0, 0, time.UTC)
	w := newCapWorld(t, CapEnforce, now)
	yesterday, today := w.addr("yday"), w.addr("tday")
	w.seed(yesterday, "sent", "campaign_update", "campaign", `{"campaign":"vsl_reeng_d1pm"}`, time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC))
	w.seed(today, "sent", "campaign_update", "campaign", `{"campaign":"vsl_reeng_d1pm"}`, time.Date(2026, 11, 21, 23, 30, 0, 0, time.UTC))
	if _, err := w.send(yesterday, "onboarding_day4", "wasbot", nil, ""); err != nil {
		t.Errorf("send after a marketing email late the previous Lagos day: refused: %v", err)
	}
	if _, err := w.send(today, "onboarding_day4", "wasbot", nil, ""); !isCapped(err) {
		t.Errorf("send after a marketing email 15 minutes into this Lagos day: want capped, got %v", err)
	}
}

// Rows that never left (failed) don't count; mixed-case rows do.
func TestMarketingCap_CountedRows_Integration(t *testing.T) {
	w := newCapWorld(t, CapEnforce, capNow)
	failed := w.addr("failed")
	w.seed(failed, "failed", "campaign_update", "campaign", `{"campaign":"x"}`, capNow.Add(-time.Hour))
	if _, err := w.send(failed, "campaign_update", "campaign", map[string]interface{}{"campaign": "y"}, ""); err != nil {
		t.Errorf("a failed row counted toward the cap: %v", err)
	}
	mixed := w.addr("mixed")
	w.seed("Mixed-"+w.run+"@Cap-Test.example", "sent", "onboarding_day1", "wasbot", `{"sequence":true}`, capNow.Add(-time.Hour))
	if _, err := w.send(mixed, "campaign_update", "campaign", map[string]interface{}{"campaign": "y"}, ""); !isCapped(err) {
		t.Errorf("a mixed-case row must count for the lower-case address: got %v", err)
	}
}

// Override: allow 2 lets the second through and refuses the third; allow 0
// bypasses. Both are stamped on the row.
func TestMarketingCap_Override_Integration(t *testing.T) {
	w := newCapWorld(t, CapEnforce, capNow)
	a := w.addr("ovr")
	ovr := func(allow int) *domain.SendEmailRequest {
		return &domain.SendEmailRequest{To: a, Template: "campaign_update", SourceService: "campaign",
			Metadata:             map[string]interface{}{"campaign": "vsl_reeng_d1pm"},
			TemplateData:         map[string]interface{}{"Subject": "s", "Body": "b", "HTMLBody": "<p>b</p>"},
			MarketingCapOverride: &domain.MarketingCapOverride{Reason: "reeng am/pm day", Allow: allow}}
	}
	if _, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "vsl_reeng_d1am"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Send(context.Background(), ovr(2)); err != nil {
		t.Fatalf("allow 2, second send: refused: %v", err)
	}
	if _, err := w.svc.Send(context.Background(), ovr(2)); !isCapped(err) {
		t.Fatalf("allow 2, third send: want capped, got %v", err)
	}
	if _, err := w.svc.Send(context.Background(), ovr(0)); err != nil {
		t.Fatalf("allow 0 (bypass): refused: %v", err)
	}
	var stamped int
	w.db.Get(&stamped, `SELECT count(*) FROM emails WHERE to_address = $1 AND metadata->'cap_override'->>'reason' = 'reeng am/pm day'`, a)
	if stamped != 2 {
		t.Errorf("%d rows carry cap_override, want 2", stamped)
	}
	bad := ovr(2)
	bad.MarketingCapOverride.Reason = " "
	if _, err := w.svc.Send(context.Background(), bad); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("override without a reason: want ErrInvalidRequest, got %v", err)
	}
}

// Shadow: the would-be refusal still sends, and the row says so.
func TestMarketingCap_Shadow_Integration(t *testing.T) {
	w := newCapWorld(t, CapShadow, capNow)
	a := w.addr("shadow")
	for i := 0; i < 2; i++ {
		if _, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "c"}, ""); err != nil {
			t.Fatalf("shadow send %d refused: %v", i, err)
		}
	}
	var would int
	w.db.Get(&would, `SELECT count(*) FROM emails WHERE to_address = $1 AND metadata->>'daily_cap' = 'would_cap'`, a)
	if w.rows(a) != 2 || would != 1 {
		t.Errorf("shadow: %d rows, %d stamped would_cap; want 2 and 1", w.rows(a), would)
	}
}

// Off: no check and no lock. A send must not wait on another holder of the
// address's lock, and nothing is stamped.
func TestMarketingCap_Off_Integration(t *testing.T) {
	w := newCapWorld(t, CapOff, capNow)
	a := w.addr("off")
	tx := w.db.MustBegin()
	defer tx.Rollback()
	tx.MustExec(`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "mktcap:"+a)

	done := make(chan error, 2)
	go func() {
		for i := 0; i < 2; i++ {
			_, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "c"}, "")
			done <- err
		}
	}()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("off-mode send %d: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("off-mode send blocked on the cap lock")
		}
	}
	var would int
	w.db.Get(&would, `SELECT count(*) FROM emails WHERE to_address = $1 AND metadata ? 'daily_cap'`, a)
	if w.rows(a) != 2 || would != 0 {
		t.Errorf("off: %d rows, %d stamped; want 2 and 0", w.rows(a), would)
	}
}

// seedNeverEnqueued writes a row as Send leaves it when the queue was down.
func (w *capWorld) seedNeverEnqueued(to, idem string, at time.Time) uuid.UUID {
	id := uuid.New()
	w.db.MustExec(`INSERT INTO emails (id, to_address, subject, body, template, status, error_message, retry_count, idempotency_id, source_service, metadata, created_at, updated_at)
		VALUES ($1, $2, 's', 'b', 'onboarding_day3', 'failed', 'Failed to enqueue', 0, $3, 'wasbot', '{"sequence":true}', $4, $4)`, id, to, idem, at)
	return id
}

func (w *capWorld) status(id uuid.UUID) string {
	var st string
	if err := w.db.Get(&st, `SELECT status FROM emails WHERE id = $1`, id); err != nil {
		w.t.Fatal(err)
	}
	return st
}

// A retry of a row that never reached the queue sends it (same row, back to
// queued, enqueued), and still under the cap; a row the worker gave up on
// stays a duplicate.
func TestMarketingCap_NeverEnqueuedRetry_Integration(t *testing.T) {
	for _, mode := range []MarketingCapMode{CapOff, CapEnforce} {
		w := newCapWorld(t, mode, capNow)
		a := w.addr("revive-" + string(mode))
		idem := "cap-" + w.run + "-revive-" + string(mode)
		id := w.seedNeverEnqueued(a, idem, capNow.Add(-time.Minute))
		resp, err := w.send(a, "onboarding_day3", "wasbot", map[string]interface{}{"sequence": true}, idem)
		if err != nil || resp.ID != id || w.status(id) != "queued" || len(w.sent) != 1 || w.rows(a) != 1 {
			t.Errorf("%s: revive: err=%v id=%v status=%s enqueued=%d rows=%d", mode, err, resp, w.status(id), len(w.sent), w.rows(a))
		}
	}

	w := newCapWorld(t, CapEnforce, capNow)
	a := w.addr("revive-capped")
	idem := "cap-" + w.run + "-revive-capped"
	id := w.seedNeverEnqueued(a, idem, capNow.Add(-time.Hour))
	if _, err := w.send(a, "campaign_update", "campaign", map[string]interface{}{"campaign": "c"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.send(a, "onboarding_day3", "wasbot", nil, idem); !isCapped(err) || w.status(id) != "failed" {
		t.Errorf("revive after today's marketing email: want capped and left failed, got %v / %s", err, w.status(id))
	}

	// Worker gave up after retries: a duplicate, not revived.
	b := w.addr("gaveup")
	gid := uuid.New()
	w.db.MustExec(`INSERT INTO emails (id, to_address, subject, body, template, status, error_message, retry_count, idempotency_id, source_service, created_at, updated_at)
		VALUES ($1, $2, 's', 'b', 'onboarding_day3', 'failed', 'provider 500', 3, $3, 'wasbot', $4, $4)`, gid, b, "cap-"+w.run+"-gaveup", capNow.Add(-time.Hour))
	before := len(w.sent)
	if resp, err := w.send(b, "onboarding_day3", "wasbot", nil, "cap-"+w.run+"-gaveup"); err != nil || resp.ID != gid || len(w.sent) != before || w.status(gid) != "failed" {
		t.Errorf("worker-failed row should stay a duplicate: %v %v", resp, err)
	}
}

// Bulk sends share one Metadata map: one shadow stamp must not mark the rest.
func TestMarketingCap_BulkMetadataNotShared_Integration(t *testing.T) {
	w := newCapWorld(t, CapShadow, capNow)
	busy, fresh := w.addr("bulkbusy"), w.addr("bulkfresh")
	w.seed(busy, "sent", "campaign_update", "campaign", `{"campaign":"x"}`, capNow.Add(-time.Hour))
	shared := map[string]interface{}{"campaign": "bulk"}
	for _, to := range []string{busy, fresh} {
		if _, err := w.send(to, "campaign_update", "campaign", shared, ""); err != nil {
			t.Fatal(err)
		}
	}
	var stamped int
	w.db.Get(&stamped, `SELECT count(*) FROM emails WHERE to_address = $1 AND metadata->>'daily_cap' = 'would_cap'`, fresh)
	if stamped != 0 || shared["daily_cap"] != nil {
		t.Errorf("the shared map was stamped (fresh stamped=%d, map=%v)", stamped, shared)
	}
}
