package test

import (
	"backend/common/proto"
	"backend/onlineconversation/internal/controller"
	"backend/onlineconversation/internal/dto"
	"backend/onlineconversation/internal/grpccontroller"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func serve(svc *MockService, method, target, userId, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	controller.NewController(svc, mux)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if userId != "" {
		req.Header.Set("X-User-Id", userId)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

var idBody = fmt.Sprintf(`{"id":"%s"}`, conversationId)

func TestController_create(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().CreateConversation(mock.Anything, memberId, mock.MatchedBy(func(r dto.CreateConversationRequest) bool {
		return r.WrittenBy == "shakespeare" && r.Capacity == 6
	})).Return(map[string]uuid.UUID{"conversationId": conversationId}, nil)

	rec := serve(svc, http.MethodPost, "/onlineconversation/create", memberId.String(),
		`{"writtenBy":"shakespeare","capacity":6}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"conversationId":"%s"}`, conversationId), rec.Body.String())
}

func TestController_rejectsBadRequestsBeforeTheService(t *testing.T) {
	tests := []struct {
		name, method, target, userId, body string
	}{
		{"create without user", http.MethodPost, "/onlineconversation/create", "", `{}`},
		{"create with bad body", http.MethodPost, "/onlineconversation/create", memberId.String(), `{bad`},
		{"delete with bad id", http.MethodDelete, "/onlineconversation/delete?id=nope", memberId.String(), ""},
		{"update with bad body", http.MethodPut, "/onlineconversation/update", memberId.String(), `{bad`},
		{"detail with bad id", http.MethodGet, "/onlineconversation/detail?id=nope", memberId.String(), ""},
		{"ban without user", http.MethodPost, "/onlineconversation/ban", "", `{}`},
		{"register with bad body", http.MethodPost, "/onlineconversation/register", memberId.String(), `{bad`},
		{"deregister without user", http.MethodPost, "/onlineconversation/deregister", "", idBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(NewMockService(t), tt.method, tt.target, tt.userId, tt.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "fail to parse", rec.Body.String())
		})
	}
}

func TestController_delegatesToTheService(t *testing.T) {
	banId := uuid.New()
	tests := []struct {
		name, method, target, body string
		expect                     func(s *MockService_Expecter)
	}{
		{"delete", http.MethodDelete, "/onlineconversation/delete?id=" + conversationId.String(), "",
			func(s *MockService_Expecter) {
				s.DeleteConversation(mock.Anything, memberId, conversationId).Return(nil)
			}},
		{"update", http.MethodPut, "/onlineconversation/update", idBody,
			func(s *MockService_Expecter) {
				s.UpdateConversation(mock.Anything, memberId, dto.UpdateConversationRequest{Id: conversationId}).Return(nil)
			}},
		{"ban", http.MethodPost, "/onlineconversation/ban",
			fmt.Sprintf(`{"conversationId":"%s","banId":"%s"}`, conversationId, banId),
			func(s *MockService_Expecter) {
				s.BanParticipant(mock.Anything, memberId, conversationId, banId).Return(nil)
			}},
		{"register", http.MethodPost, "/onlineconversation/register", idBody,
			func(s *MockService_Expecter) {
				s.RegisterOnlineConversation(mock.Anything, memberId, conversationId).Return(nil)
			}},
		{"deregister", http.MethodPost, "/onlineconversation/deregister", idBody,
			func(s *MockService_Expecter) {
				s.DeregisterOnlineConversation(mock.Anything, memberId, conversationId).Return(nil)
			}},
		{"schedule notification", http.MethodPost, "/onlineconversation/notification/schedule", idBody,
			func(s *MockService_Expecter) {
				s.ScheduleNotification(mock.Anything, memberId, conversationId).Return(nil)
			}},
		{"cancel notification", http.MethodPost, "/onlineconversation/notification/cancel", idBody,
			func(s *MockService_Expecter) {
				s.CancelNotification(mock.Anything, memberId, conversationId).Return(nil)
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockService(t)
			tt.expect(svc.EXPECT())

			rec := serve(svc, tt.method, tt.target, memberId.String(), tt.body)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestController_serviceErrorIsBadRequest(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().RegisterOnlineConversation(mock.Anything, memberId, conversationId).Return(fmt.Errorf("already fully registered"))

	rec := serve(svc, http.MethodPost, "/onlineconversation/register", memberId.String(), idBody)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "already fully registered", rec.Body.String())
}

func TestController_detailAndTurnAreJSON(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().GetConversationDetail(mock.Anything, conversationId, memberId).
		Return(&dto.OnlineConversationDetailResponse{Novel: "Hamlet", CanEnter: true}, nil)
	svc.EXPECT().GenerateTurn().Return(&dto.GetTurnResponse{Username: "123", Credential: "abc"})

	detail := serve(svc, http.MethodGet, "/onlineconversation/detail?id="+conversationId.String(), memberId.String(), "")
	turn := serve(svc, http.MethodGet, "/onlineconversation/turn", memberId.String(), "")

	require.Equal(t, http.StatusOK, detail.Code)
	assert.Contains(t, detail.Body.String(), `"novel":"Hamlet"`)
	require.Equal(t, http.StatusOK, turn.Code)
	assert.Contains(t, turn.Body.String(), `"credential":"abc"`)
}

func TestGRPCController_relaySignalWithoutConnectedMembers(t *testing.T) {
	gc := grpccontroller.NewGRPCController(controller.NewController(NewMockService(t), http.NewServeMux()))
	other := uuid.New()

	res, err := gc.RelaySignal(context.Background(), &proto.RelaySignalRequest{
		FromId: memberId[:], ToIds: [][]byte{other[:]}, Signal: []byte(`{}`),
	})

	assert.NoError(t, err)
	assert.Nil(t, res)
}
