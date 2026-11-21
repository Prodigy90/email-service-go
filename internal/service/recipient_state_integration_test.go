package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/prodigy90/email-service-go/internal/db"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
)

// TestRecipientState_Integration runs against a real Postgres. Point
// EMAIL_SERVICE_TEST_DATABASE_URL at a scratch database: an empty one gets the
// schema from migrations/, one restored from `pg_dump -s` of prod is used as
// is. Rows are keyed on a per-run address suffix and removed afterwards.
// Skipped when the variable is unset (CI has no Postgres).
func TestRecipientState_Integration(t *testing.T) {
	dsn := os.Getenv("EMAIL_SERVICE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("EMAIL_SERVICE_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	sqlxDB, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlxDB.Close() })
	var hasEmails bool
	if err := sqlxDB.Get(&hasEmails, `SELECT to_regclass('public.emails') IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if !hasEmails {
		if err := db.RunMigrations(sqlxDB.DB, "../../migrations"); err != nil {
			t.Fatal(err)
		}
		t.Log("schema built from migrations/")
	} else {
		t.Log("schema already present (e.g. restored from a prod pg_dump -s)")
	}

	run := uuid.NewString()[:8]
	addr := func(name string) string { return fmt.Sprintf("%s-%s@journey-test.example", name, run) }
	unsub, bounce, busy, seq, empty, other := addr("unsub"), addr("bounce"), addr("busy"), addr("seq"), addr("empty"), addr("other")
	t.Cleanup(func() {
		like := "%-" + run + "@journey-test.example"
		sqlxDB.MustExec(`DELETE FROM emails WHERE to_address LIKE $1`, like)
		sqlxDB.MustExec(`DELETE FROM suppressions WHERE lower(email) LIKE $1`, like)
	})

	// now = 11:00 Lagos on 21 Nov, so the Lagos day began at 23:00 UTC on 20 Nov.
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	send := func(to, status, template, metadata string, created time.Time) {
		t.Helper()
		var tmpl any
		if template != "" {
			tmpl = template
		}
		sqlxDB.MustExec(`INSERT INTO emails (id, to_address, subject, body, template, status, source_service, metadata, created_at, updated_at)
			VALUES ($1, $2, 's', 'b', $3, $4, 'test', $5::jsonb, $6, $6)`,
			uuid.New(), to, tmpl, status, metadata, created)
	}
	suppress := func(email, reason, source string) {
		sqlxDB.MustExec(`INSERT INTO suppressions (email, reason, source) VALUES ($1, $2, $3)`, email, reason, source)
	}

	suppress(unsub, "unsubscribe", "user")
	// The bounce webhook stores the address in the case it was sent with.
	suppress("Bounce-"+run+"@Journey-Test.example", "bounce", "webhook")
	send(bounce, "bounced", "campaign_update", `{"campaign":"old_promo"}`, at("2026-10-01T09:00:00Z"))

	// busy: two accepted sends today, one yesterday (inside 24h), one failed today.
	// 23:30 UTC on the 20th is 00:30 Lagos on the 21st: today in Lagos, not in UTC.
	send(busy, "sent", "campaign_update", `{"campaign":"vsl_reeng_d3am"}`, at("2026-11-20T23:30:00Z"))
	send(busy, "sent", "payment_success", `{}`, at("2026-11-21T09:00:00Z"))
	send(busy, "sent", "campaign_update", `{"campaign":"vsl_reeng_d2pm"}`, at("2026-11-20T20:00:00Z"))
	send(busy, "failed", "campaign_update", `{"campaign":"vsl_reeng_d3pm"}`, at("2026-11-21T09:30:00Z"))

	send(seq, "queued", "trial_day3", `{"sequence":true}`, at("2026-11-21T08:00:00Z"))
	// Never requested: must not leak into anyone's counts.
	send(other, "sent", "campaign_update", `{"campaign":"x"}`, at("2026-11-21T09:00:00Z"))

	svc := NewRecipientStateService(postgres.NewRecipientStateRepository(sqlxDB))
	svc.now = func() time.Time { return now }
	resp, err := svc.Get(ctx, []string{" " + unsub + " ", "BOUNCE-" + run + "@journey-test.example", busy, seq, empty, busy})
	if err != nil {
		t.Fatal(err)
	}

	if want := at("2026-11-20T23:00:00Z"); !resp.DayStart.Equal(want) {
		t.Errorf("day_start = %s, want %s", resp.DayStart, want)
	}
	if len(resp.Recipients) != 5 {
		t.Fatalf("got %d recipients, want 5 (deduped, in order)", len(resp.Recipients))
	}
	byEmail := map[string]RecipientState{}
	for _, r := range resp.Recipients {
		byEmail[r.Email] = r
	}
	if resp.Recipients[0].Email != unsub || resp.Recipients[4].Email != empty {
		t.Errorf("order wrong: %s ... %s", resp.Recipients[0].Email, resp.Recipients[4].Email)
	}

	u := byEmail[unsub]
	if !u.Suppressed || !u.Unsubscribed || u.Bounced || u.Suppression == nil || u.Suppression.Source != "user" {
		t.Errorf("unsub: %+v", u)
	}

	b := byEmail[bounce]
	if !b.Suppressed || !b.Bounced || b.Unsubscribed || !b.EverBounced {
		t.Errorf("bounce (mixed-case suppression row): %+v", b)
	}
	if b.SendsToday != 0 || b.LastSend == nil || b.LastSend.Campaign == nil || *b.LastSend.Campaign != "old_promo" {
		t.Errorf("bounce sends: %+v", b)
	}

	x := byEmail[busy]
	if x.Suppressed {
		t.Errorf("busy should not be suppressed")
	}
	if x.SendsToday != 2 || x.MarketingToday != 1 || x.Sends24h != 3 {
		t.Errorf("busy counts today=%d marketing=%d 24h=%d, want 2/1/3", x.SendsToday, x.MarketingToday, x.Sends24h)
	}
	if x.LastSend == nil || x.LastSend.Template == nil || *x.LastSend.Template != "payment_success" || !x.LastSend.Service {
		t.Errorf("busy last_send: %+v", x.LastSend)
	}
	if x.LastMarketingSend == nil || x.LastMarketingSend.Campaign == nil || *x.LastMarketingSend.Campaign != "vsl_reeng_d3am" || x.LastMarketingSend.Service {
		t.Errorf("busy last_marketing_send: %+v", x.LastMarketingSend)
	}

	s := byEmail[seq]
	if s.LastMarketingSend == nil || !s.LastMarketingSend.Sequence || s.MarketingToday != 1 {
		t.Errorf("seq: %+v", s)
	}

	e := byEmail[empty]
	if e.Suppressed || e.SendsToday != 0 || e.Sends24h != 0 || e.LastSend != nil || e.LastMarketingSend != nil || e.EverBounced {
		t.Errorf("empty should have no state: %+v", e)
	}
}

// Over 500 distinct addresses is refused before any read.
func TestRecipientState_BatchLimit(t *testing.T) {
	svc := NewRecipientStateService(nil)
	many := make([]string, MaxRecipientStateBatch+1)
	for i := range many {
		many[i] = fmt.Sprintf("a%d@x.co", i)
	}
	if _, err := svc.Get(context.Background(), many); err == nil {
		t.Fatal("want an error over the batch limit")
	}
}

func TestDayStart_Lagos(t *testing.T) {
	cases := []struct{ now, want string }{
		{"2026-11-20T22:59:59Z", "2026-11-19T23:00:00Z"},
		{"2026-11-20T23:00:00Z", "2026-11-20T23:00:00Z"},
		{"2026-11-21T10:00:00Z", "2026-11-20T23:00:00Z"},
	}
	for _, c := range cases {
		n, _ := time.Parse(time.RFC3339, c.now)
		w, _ := time.Parse(time.RFC3339, c.want)
		if got := DayStart(n); !got.Equal(w) {
			t.Errorf("DayStart(%s) = %s, want %s", c.now, got.UTC(), c.want)
		}
	}
}
