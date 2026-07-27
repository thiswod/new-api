package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSendEmailChallengeDoesNotRequireTurnstileButRegisterDoes(t *testing.T) {
	originalTurnstileEnabled := common.TurnstileCheckEnabled
	originalUserSendEmailEnabled := common.UserSendEmailVerificationEnabled
	common.TurnstileCheckEnabled = true
	common.UserSendEmailVerificationEnabled = false
	t.Cleanup(func() {
		common.TurnstileCheckEnabled = originalTurnstileEnabled
		common.UserSendEmailVerificationEnabled = originalUserSendEmailEnabled
	})

	engine := newAPIRouterForTest(t)

	challenge := performAPIRequest(engine, http.MethodPost, "/api/user_send_email/challenge", []byte(`{"email":"person@example.com"}`), "192.0.2.101:1234")
	assert.Equal(t, http.StatusOK, challenge.Code)
	assert.Contains(t, challenge.Body.String(), "用户发信验证未启用")
	assert.NotContains(t, challenge.Body.String(), "Turnstile token 为空")

	register := performAPIRequest(engine, http.MethodPost, "/api/user/register", []byte(`{}`), "192.0.2.102:1234")
	assert.Equal(t, http.StatusOK, register.Code)
	assert.Contains(t, register.Body.String(), "Turnstile token 为空")
}

func TestUserSendEmailChallengeKeepsRateAndBodyLimits(t *testing.T) {
	originalTurnstileEnabled := common.TurnstileCheckEnabled
	originalUserSendEmailEnabled := common.UserSendEmailVerificationEnabled
	originalBodyLimit := constant.AnonymousRequestBodyLimitKB
	common.TurnstileCheckEnabled = true
	common.UserSendEmailVerificationEnabled = false
	constant.AnonymousRequestBodyLimitKB = 1
	t.Cleanup(func() {
		common.TurnstileCheckEnabled = originalTurnstileEnabled
		common.UserSendEmailVerificationEnabled = originalUserSendEmailEnabled
		constant.AnonymousRequestBodyLimitKB = originalBodyLimit
	})

	engine := newAPIRouterForTest(t)
	body := []byte(`{"email":"person@example.com"}`)
	for i := 0; i < 2; i++ {
		response := performAPIRequest(engine, http.MethodPost, "/api/user_send_email/challenge", body, "192.0.2.103:1234")
		assert.Equal(t, http.StatusOK, response.Code)
	}
	rateLimited := performAPIRequest(engine, http.MethodPost, "/api/user_send_email/challenge", body, "192.0.2.103:1234")
	assert.Equal(t, http.StatusTooManyRequests, rateLimited.Code)

	tooLarge := performAPIRequest(engine, http.MethodPost, "/api/user_send_email/challenge", bytes.Repeat([]byte("x"), 1025), "192.0.2.104:1234")
	assert.Equal(t, http.StatusRequestEntityTooLarge, tooLarge.Code)
}

func newAPIRouterForTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NoError(t, engine.SetTrustedProxies(nil))
	SetApiRouter(engine)
	return engine
}

func performAPIRequest(engine http.Handler, method, path string, body []byte, remoteAddr string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
