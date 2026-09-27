package test

import (
	"backend/notification/internal/controller"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func serve(t *testing.T, svc *MockService, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	controller.SetController(svc, mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func registerRequest(userId, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/notification/register", strings.NewReader(body))
	req.Header.Set("X-User-Id", userId)
	return req
}

func TestRegisterNotification_ok(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().RegisterNotification(mock.Anything, memberId, "ios", "tok").Return(nil)

	rec := serve(t, svc, registerRequest(memberId.String(), `{"os":"ios","devicePushToken":"tok"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegisterNotification_serviceErrorIsBadRequest(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().RegisterNotification(mock.Anything, memberId, "ios", "tok").Return(errors.New("cassandra down"))

	rec := serve(t, svc, registerRequest(memberId.String(), `{"os":"ios","devicePushToken":"tok"}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "cassandra down", rec.Body.String())
}

func TestRegisterNotification_badRequestsDoNotReachTheService(t *testing.T) {
	for name, req := range map[string]*http.Request{
		"bad user id": registerRequest("not-a-uuid", `{"os":"ios","devicePushToken":"tok"}`),
		"bad body":    registerRequest(memberId.String(), `{bad`),
	} {
		t.Run(name, func(t *testing.T) {
			rec := serve(t, NewMockService(t), req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "incorrect body", rec.Body.String())
		})
	}
}

func TestRegisterNotification_wrongMethodIsNotRouted(t *testing.T) {
	rec := serve(t, NewMockService(t), httptest.NewRequest(http.MethodGet, "/notification/register", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
