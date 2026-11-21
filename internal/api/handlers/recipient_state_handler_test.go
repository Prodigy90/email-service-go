package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/prodigy90/email-service-go/internal/service"
)

// Input checks run before any read, so a nil repository proves none was reached.
func TestRecipientState_RejectsBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRecipientStateHandler(service.NewRecipientStateService(nil), zerolog.Nop())
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	many := `{"emails":["a@x.co"` + strings.Repeat(`,"a@x.co"`, 500) + `]}`
	for name, body := range map[string]string{"empty": `{}`, "no emails": `{"emails":[]}`, "not json": `emails=a`, "over 500": many} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/recipients/state", bytes.NewBufferString(body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
			}
		})
	}
}
