package templates_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// Welcome refresh (2026-10-12): the signup welcome (onboarding_day0), the
// paid series (onboarding_paid_day1-7), the first-payment welcome folded into
// payment_success_onetime, and a "Tomorrow: ..." line on every series email
// but the last. Each template is rendered against every data state the
// backend produces for it.

// helpLink is the preset WhatsApp chat, never a bare number to copy.
const helpLink = "https://wa.me/2349018943768?text=Hi%20Victor%2C%20I%20just%20joined%20WASBOT%20and%20need%20help."

// signoffs are the two house sign-offs: Victor on the welcomes, the team on
// the series.
var signoffs = []string{"Victor", "The WASBOT Team"}

// countSignoffs returns how many sign-offs the plaintext and the HTML carry.
// "The WASBOT Team" is counted anywhere (older templates sign "— The WASBOT
// Team"); "Victor" only as a whole line, so "Hi%20Victor" in the help link
// never counts.
func countSignoffs(body, html string) [2]int {
	var n [2]int
	n[0] = strings.Count(body, "The WASBOT Team")
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "Victor" {
			n[0]++
		}
	}
	for _, s := range signoffs {
		n[1] += strings.Count(html, "<strong>"+s+"</strong></p>")
	}
	return n
}

// teasers are the exact "next email" lines, one per series email. The last
// email of each series has none.
var teasers = map[string]string{
	"onboarding_day0":          "Tomorrow: how to get every new chat saved to your contacts, without typing a thing.",
	"onboarding_day1":          "Tomorrow: how to post your status from the cloud, with your phone in your pocket.",
	"onboarding_unlinked_day1": "Tomorrow: how to post your status from the cloud, with your phone in your pocket.",
	"onboarding_day2":          "Tomorrow: how WASBOT replies to your customers while you sleep.",
	"onboarding_unlinked_day2": "Tomorrow: how WASBOT replies to your customers while you sleep.",
	"onboarding_day3":          "Tomorrow: how to send one message to all your groups at once.",
	"onboarding_unlinked_day3": "Tomorrow: how to send one message to all your groups at once.",
	"onboarding_day4":          "Tomorrow: how to keep today's price for as long as you stay.",
	"onboarding_unlinked_day4": "Tomorrow: how to keep today's price for as long as you stay.",
	"onboarding_day5":          "Tomorrow: what you get if you decide before your trial ends.",
	"onboarding_unlinked_day5": "Tomorrow: what you get if you decide before your trial ends.",
	"onboarding_day6":          "Next, and last: a short reminder a few hours before your trial ends.",
	"onboarding_unlinked_day6": "Next, and last: a short reminder a few hours before your trial ends.",
	"onboarding_day7":          "",
	"onboarding_unlinked_day7": "",
	"onboarding_paid_day1":     "Tomorrow: how to post your status from the cloud, with your phone in your pocket.",
	"onboarding_paid_day2":     "Tomorrow: how WASBOT replies to your customers while you sleep.",
	"onboarding_paid_day3":     "Tomorrow: how to send one message to all your groups at once.",
	"onboarding_paid_day4":     "Tomorrow: how to follow up with customers for days, without remembering to.",
	"onboarding_paid_day5":     "Tomorrow: how to see who actually watches your status.",
	"onboarding_paid_day6":     "Tomorrow: where to get help anytime you're stuck, and where to learn more.",
	"onboarding_paid_day7":     "",
}

var paidTemplates = []string{
	"onboarding_paid_day1", "onboarding_paid_day2", "onboarding_paid_day3", "onboarding_paid_day4",
	"onboarding_paid_day5", "onboarding_paid_day6", "onboarding_paid_day7",
}

type renderState struct {
	name string
	data map[string]interface{}
}

// day0States mirror what reaches onboarding_day0: SendWelcome at signup (with
// and without a usable first name) and the sequence cron, which sends day0 as
// the first re-engagement email with no trial and no code.
func day0States() []renderState {
	welcome := func() map[string]interface{} {
		return withKeys(baseLifecycleData(), map[string]interface{}{
			"FirstName": "Ada", "Code": "ADA10", "CodePct": 10, "TrialEnd": "Friday, 21 August",
			"VideoURL": "https://youtu.be/eYCPi68WpKM", "VideoTitle": "Watch the full setup (9 min)",
		})
	}
	noName := welcome()
	delete(noName, "FirstName")
	noName["Name"] = "there"
	codeSpent := welcome()
	delete(codeSpent, "Code")
	delete(codeSpent, "CodePct")
	// Missing optionals: no first name, no video, no group link, no code.
	bare := map[string]interface{}{
		"Name": "there", "DashboardURL": "https://www.wasbot.app/dashboard",
		"ConnectURL": "https://www.wasbot.app/dashboard", "TrialEnd": "Friday, 21 August",
	}
	reengage := withKeys(baseLifecycleData(), map[string]interface{}{
		"ContactsSaved": 0, "StatusesPosted": 0, "StatusRecipients": 0,
		"StatusViews": 0, "GroupSends": 0, "SequencesActive": 0,
	})
	return []renderState{
		{"signup", welcome()}, {"signup-no-first-name", noName}, {"signup-code-spent", codeSpent},
		{"missing-optional", bare}, {"reengagement", reengage},
	}
}

// paidStates mirror the cron's data for a paid-series reader: no trial or
// offer fields ever (the cron attaches those only to an active trial).
func paidStates() []renderState {
	fullStats := map[string]interface{}{
		"ContactsSaved": 3412, "StatusesPosted": 18, "StatusRecipients": 41200,
		"StatusViews": 9633, "GroupSends": 22, "SequencesActive": 3,
		"HasActivity": true, "TimeSavedHours": 27,
	}
	zeroStats := map[string]interface{}{
		"ContactsSaved": 0, "StatusesPosted": 0, "StatusRecipients": 0,
		"StatusViews": 0, "GroupSends": 0, "SequencesActive": 0,
	}
	bare := map[string]interface{}{
		"Name": "there", "DashboardURL": "https://www.wasbot.app/dashboard",
		"ConnectURL": "https://www.wasbot.app/dashboard",
	}
	return []renderState{
		{"paid-active", withKeys(baseLifecycleData(), fullStats)},
		{"paid-never-linked", withKeys(withKeys(baseLifecycleData(), zeroStats), map[string]interface{}{"NeedsLink": true})},
		{"paid-stats-unavailable", baseLifecycleData()},
		{"missing-optional", bare},
	}
}

// receiptStates mirror the webhook's payment_success_onetime data: a pay-first
// buyer's first payment (paid series follows), a trial converter's first
// payment (no series follows: the trial series already ran), a renewal, and a
// first payment with every optional field missing.
func receiptStates() []renderState {
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"CustomerName": "Ada", "FirstName": "Ada", "ProductName": "Basic", "Amount": "₦8,000.00",
			"Currency": "NGN", "Duration": "1 month", "ExpiryDate": "November 12, 2026",
			"TransactionID": "T123", "DashboardURL": "https://www.wasbot.app/dashboard",
		}
	}
	payFirst := withKeys(base(), map[string]interface{}{"FirstPayment": true, "PaidSeriesFollows": true})
	converter := withKeys(base(), map[string]interface{}{"FirstPayment": true})
	renewal := base()
	bare := withKeys(base(), map[string]interface{}{"FirstPayment": true, "CustomerName": "there"})
	delete(bare, "FirstName")
	delete(bare, "DashboardURL")
	bare["ExpiryDate"] = ""
	return []renderState{
		{"pay-first", payFirst}, {"trial-converter", converter},
		{"renewal", renewal}, {"first-missing-optional", bare},
	}
}

var (
	wrongCasing = regexp.MustCompile(`Wasbot|WasBot|wasBot`)
	// A subject must never greet the fallback ("Welcome, there").
	fallbackInSubject = regexp.MustCompile(`, there\b|,\s*\.`)
)

// assertClean is the shared bar for every touched template.
func assertClean(t *testing.T, label, subject, body, html string) {
	t.Helper()
	for part, out := range map[string]string{"subject": subject, "body": body, "html": html} {
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s %s: leaked <no value>", label, part)
		}
		if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
			t.Errorf("%s %s: unrendered template action", label, part)
		}
		if strings.Contains(out, "—") {
			t.Errorf("%s %s: em dash (house emails never use one)", label, part)
		}
		if m := wrongCasing.FindString(out); m != "" {
			t.Errorf("%s %s: %q, the product is WASBOT", label, part, m)
		}
	}
	if fallbackInSubject.MatchString(subject) {
		t.Errorf("%s: subject greets a missing name: %q", label, subject)
	}
}

// assertHelpLink also guarantees the body carries a working link at all.
func assertHelpLink(t *testing.T, label, body, html string) {
	t.Helper()
	if !strings.Contains(body, helpLink) || !strings.Contains(html, helpLink) {
		t.Errorf("%s: missing the preset wa.me help link", label)
	}
}

// assertTeaser checks the "next email" line: present in both branches when
// expected, and no stray "Tomorrow:" line when not.
func assertTeaser(t *testing.T, label, want, body, html string) {
	t.Helper()
	if want == "" {
		if strings.Contains(body, "Tomorrow:") || strings.Contains(html, "Tomorrow:") ||
			strings.Contains(body, "Next, and last:") || strings.Contains(html, "Next, and last:") {
			t.Errorf("%s: the last email of a series must not tease a next one", label)
		}
		return
	}
	if !strings.Contains(body, want) {
		t.Errorf("%s: plaintext is missing the teaser %q", label, want)
	}
	head, rest, _ := strings.Cut(want, ":")
	if !strings.Contains(html, "<strong>"+head+":</strong>"+rest) {
		t.Errorf("%s: HTML is missing the teaser %q", label, want)
	}
	// The teaser is the LAST line of the plaintext.
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if got := strings.TrimSpace(lines[len(lines)-1]); got != want {
		t.Errorf("%s: plaintext ends with %q, want the teaser last", label, got)
	}
}

func TestWelcome_Day0RendersInEveryState(t *testing.T) {
	ts := newTestTemplateService(t)
	for _, st := range day0States() {
		label := "onboarding_day0 [" + st.name + "]"
		subject, body, html, err := ts.RenderWithBranding("onboarding_day0", st.data, domain.DefaultBranding())
		if err != nil {
			t.Fatalf("%s: render failed: %v", label, err)
		}
		assertClean(t, label, subject, body, html)
		assertHelpLink(t, label, body, html)
		assertTeaser(t, label, teasers["onboarding_day0"], body, html)
		group, ok := st.data["WhatsAppGroupURL"].(string)
		if !ok {
			group = "https://chat.whatsapp.com/EoP7wlsiUe4CKKA5E6BXCY" // template fallback
		}
		for _, must := range []string{
			"https://youtu.be/eYCPi68WpKM", group,
			"https://whatsapp.com/channel/0029VbDZGfB17EmsMzdM8f31",
			"https://instagram.com/wasbot.app", "https://x.com/wasbot", "https://www.youtube.com/@wasbot_app",
		} {
			// Text links, not only footer icons: every one is in the plaintext.
			if !strings.Contains(body, must) || !strings.Contains(html, must) {
				t.Errorf("%s: missing %s", label, must)
			}
		}
		// One video only: the 9-minute setup covers linking AND contact saving,
		// so a second contacts link would be a duplicate (founder 2026-10-12).
		if strings.Contains(body, "/watch/contacts") || strings.Contains(html, "/watch/contacts") {
			t.Errorf("%s: day0 carries a second (contacts) video link", label)
		}
		// The trial lines only appear while a trial is running.
		_, hasTrial := st.data["TrialEnd"]
		if got := strings.Contains(body, "7 day free trial"); got != hasTrial {
			t.Errorf("%s: trial line shown=%v, want %v", label, got, hasTrial)
		}
		_, hasCode := st.data["Code"]
		if got := strings.Contains(body, "discount code"); got != hasCode {
			t.Errorf("%s: code paragraph shown=%v, want %v", label, got, hasCode)
		}
		// The code paragraph must say what makes it worth using now: any plan,
		// any period, trial-only, full price after.
		if hasCode {
			for _, must := range []string{"any plan", "a full year", "only works while your free trial is active", "full price"} {
				if !strings.Contains(body, must) || !strings.Contains(html, must) {
					t.Errorf("%s: code paragraph missing %q", label, must)
				}
			}
		}
		if fn, ok := st.data["FirstName"]; ok && subject != "Welcome to WASBOT, "+fn.(string)+". Start here" {
			t.Errorf("%s: subject %q", label, subject)
		}
		if _, ok := st.data["FirstName"]; !ok && subject != "Welcome to WASBOT. Start here" {
			t.Errorf("%s: subject %q", label, subject)
		}
	}
}

func TestWelcome_PaidSeriesRendersInEveryState(t *testing.T) {
	ts := newTestTemplateService(t)
	for _, name := range paidTemplates {
		for _, st := range paidStates() {
			label := name + " [" + st.name + "]"
			subject, body, html, err := ts.RenderWithBranding(name, st.data, domain.DefaultBranding())
			if err != nil {
				t.Fatalf("%s: render failed: %v", label, err)
			}
			assertClean(t, label, subject, body, html)
			assertHelpLink(t, label, body, html)
			assertTeaser(t, label, teasers[name], body, html)
			if got := countSignoffs(body, html); got != [2]int{1, 1} {
				t.Errorf("%s: sign-offs (plain, html) = %v, want one each", label, got)
			}
			// Paid-safe: nothing trial-, code- or upgrade-shaped.
			low := strings.ToLower(body + html)
			for _, banned := range []string{"trial", "discount", "% off", "upgrade", "price lock", "locked forever", "₦", "$9"} {
				if strings.Contains(low, banned) {
					t.Errorf("%s: paid copy mentions %q", label, banned)
				}
			}
			_, needsLink := st.data["NeedsLink"]
			if got := strings.Contains(body, "Not linked yet?"); got != needsLink {
				t.Errorf("%s: link nudge shown=%v, want %v", label, got, needsLink)
			}
		}
	}
}

func TestWelcome_FirstPaymentReceipt(t *testing.T) {
	ts := newTestTemplateService(t)
	for _, st := range receiptStates() {
		label := "payment_success_onetime [" + st.name + "]"
		subject, body, html, err := ts.RenderWithBranding("payment_success_onetime", st.data, domain.DefaultBranding())
		if err != nil {
			t.Fatalf("%s: render failed: %v", label, err)
		}
		assertClean(t, label, subject, body, html)

		first, _ := st.data["FirstPayment"].(bool)
		follows, _ := st.data["PaidSeriesFollows"].(bool)
		if got := strings.Contains(body, "Welcome to WASBOT"); got != first {
			t.Errorf("%s: welcome shown=%v, want %v (renewals keep the plain receipt)", label, got, first)
		}
		if first {
			assertHelpLink(t, label, body, html)
			for _, must := range []string{"Transaction ID: T123", "Amount paid: ₦8,000.00", "will not renew automatically"} {
				if !strings.Contains(body, must) {
					t.Errorf("%s: the welcome dropped the receipt line %q", label, must)
				}
			}
		}
		want := ""
		if follows {
			want = teasers["onboarding_day0"]
		}
		assertTeaser(t, label, want, strings.TrimSpace(body), html)
		// The dashboard button must resolve even when the caller passes no
		// DashboardURL (it used to render href="<no value>" on every receipt).
		if !strings.Contains(html, `href="https://`) {
			t.Errorf("%s: no resolvable button link", label)
		}
	}
}

// Every touched trial email keeps rendering cleanly with its teaser in all the
// states the cron produces (lifecycleStates in lifecycle_render_test.go).
func TestWelcome_TrialSeriesTeasers(t *testing.T) {
	ts := newTestTemplateService(t)
	for name, want := range teasers {
		if strings.HasPrefix(name, "onboarding_paid_") || name == "onboarding_day0" {
			continue
		}
		for _, st := range lifecycleStates() {
			label := name + " [" + st.name + "]"
			subject, body, html, err := ts.RenderWithBranding(name, st.data, domain.DefaultBranding())
			if err != nil {
				t.Fatalf("%s: render failed: %v", label, err)
			}
			assertClean(t, label, subject, body, html)
			assertTeaser(t, label, want, body, html)
		}
	}
}

// The help line is a link only: the founder's number never appears as text to
// copy, and nobody is promised "a real person" (founder 2026-10-12).
func TestWelcome_HelpLineIsLinkOnly(t *testing.T) {
	ts := newTestTemplateService(t)
	check := func(label, body, html string) {
		for _, banned := range []string{"0901 894 3768", "09018943768", "real person replies"} {
			if strings.Contains(body, banned) || strings.Contains(html, banned) {
				t.Errorf("%s: help line contains %q", label, banned)
			}
		}
		if !strings.Contains(body, "representative will respond as soon as possible") {
			t.Errorf("%s: help line missing the representative wording", label)
		}
	}
	for _, st := range day0States() {
		_, b, h, err := ts.RenderWithBranding("onboarding_day0", st.data, domain.DefaultBranding())
		if err != nil {
			t.Fatal(err)
		}
		check("onboarding_day0 ["+st.name+"]", b, h)
	}
	for _, name := range paidTemplates {
		_, b, h, err := ts.RenderWithBranding(name, paidStates()[0].data, domain.DefaultBranding())
		if err != nil {
			t.Fatal(err)
		}
		check(name, b, h)
	}
	for _, st := range receiptStates() {
		if first, _ := st.data["FirstPayment"].(bool); !first {
			continue
		}
		_, b, h, err := ts.RenderWithBranding("payment_success_onetime", st.data, domain.DefaultBranding())
		if err != nil {
			t.Fatal(err)
		}
		check("payment_success_onetime ["+st.name+"]", b, h)
	}
}

// Stats print with thousands separators. The data arrives as JSON numbers
// (float64) in production, so render both shapes.
func TestWelcome_StatsUseThousandsSeparators(t *testing.T) {
	ts := newTestTemplateService(t)
	for _, v := range []interface{}{3412, float64(3412)} {
		data := paidStates()[0].data
		data["ContactsSaved"] = v
		_, b, h, err := ts.RenderWithBranding("onboarding_paid_day1", data, domain.DefaultBranding())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b, "3,412 contacts") || !strings.Contains(h, "3,412") || strings.Contains(b, "3412") {
			t.Errorf("ContactsSaved %T: not rendered as 3,412", v)
		}
	}
}
