package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prodigy90/email-service-go/internal/service"
)

func TestWriteSendError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry := time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC)
	cases := []struct {
		err  error
		code int
		body string
	}{
		{&service.ErrMarketingCapped{To: "a@b.co", MarketingToday: 1, Cap: 1, RetryAfter: retry}, http.StatusConflict, `"code":"marketing_daily_cap"`},
		{fmt.Errorf("%w: boom", service.ErrMarketingCapUnavailable), http.StatusServiceUnavailable, `"code":"marketing_cap_unavailable"`},
		{fmt.Errorf("%w: x", service.ErrInvalidRequest), http.StatusBadRequest, `invalid request`},
		{errors.New("template render failed"), http.StatusInternalServerError, `template render failed`},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		writeSendError(ctx, c.err)
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.body) {
			t.Errorf("%v: %d %s, want %d containing %s", c.err, w.Code, w.Body.String(), c.code, c.body)
		}
	}
	// pkg/client surfaces only "error": it must start with the code.
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	writeSendError(ctx, &service.ErrMarketingCapped{To: "a@b.co", MarketingToday: 1, Cap: 1, RetryAfter: retry})
	var body CapRefusal
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !strings.HasPrefix(body.Error, "marketing_daily_cap:") || !body.RetryAfter.Equal(retry) {
		t.Errorf("refusal body %+v", body)
	}
}
