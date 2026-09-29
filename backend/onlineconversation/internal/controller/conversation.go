package controller

import (
	"backend/common/payload"
	"backend/onlineconversation/internal/dto"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

func conversationRouter(c *Controller) {
	c.Router(POST, "/onlineconversation/create", c.createConversation)
	c.Router(DELETE, "/onlineconversation/delete", c.deleteConversation)
	c.Router(PUT, "/onlineconversation/update", c.updateConversation)
	c.Router(GET, "/onlineconversation/join", c.joinConversation)
	c.Router(GET, "/onlineconversation/detail", c.getConversationDetail)
	c.Router(POST, "/onlineconversation/ban", c.banParticipant)
	c.Router(POST, "/onlineconversation/register", c.registerConversation)
	c.Router(POST, "/onlineconversation/deregister", c.deregisterConversation)
	c.Router(GET, "/onlineconversation/turn", c.getTurn)
	c.Router(POST, "/onlineconversation/notification/schedule", c.scheduleNotification)
	c.Router(POST, "/onlineconversation/notification/cancel", c.cancelNotification)
	c.Router(POST, "/onlineconversation/report", c.reportConversation)
}

func (c *Controller) createConversation(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse userId from X-User-Id header",
			"err", err,
		)
		handleError(w, errors.New("fail to parse"))
		return
	}
	var req dto.CreateConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Info("incorrect body",
			"err", err,
		)
		handleError(w, errors.New("fail to parse"))
		return
	}
	result, err := c.service.CreateConversation(
		r.Context(),
		memberId,
		req,
	)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body", "err", err)
	}
}

func (c *Controller) deleteConversation(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse userId from X-User-Id header",
			"err", err,
		)
		handleError(w, errors.New("fail to parse"))
		return
	}
	conversationId, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		slog.Error("fail to parse conversation uuid from raw string", "err", err)
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.DeleteConversation(r.Context(), memberId, conversationId)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) updateConversation(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse userId from X-User-Id header",
			"err", err,
		)
		handleError(w, errors.New("fail to parse"))
		return
	}
	var req dto.UpdateConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Info("incorrect body",
			"err", err,
		)
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.UpdateConversation(r.Context(), memberId, req)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) getConversationDetail(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse member id from raw string",
			"err", err)
		handleError(w, errors.New("fail to parse"))
		return
	}
	conversationId, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		slog.Error("fail to parse conversation uuid from raw string", "err", err)
		handleError(w, errors.New("fail to parse"))
		return
	}
	result, err := c.service.GetConversationDetail(r.Context(), conversationId, memberId)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) banParticipant(w http.ResponseWriter, r *http.Request) {
	memberIdRaw := r.Header.Get("X-User-Id")
	memberId, err := uuid.Parse(memberIdRaw)
	if err != nil {
		slog.Error("fail to parse member id from raw string",
			"err", err,
			"memberIdRaw", memberIdRaw)
		handleError(w, errors.New("fail to parse"))
		return
	}
	var req dto.BanParticipantRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.BanParticipant(r.Context(), memberId, req.ConversationId, req.BanId)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) registerConversation(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse member id from raw string",
			"err", err)
		handleError(w, errors.New("fail to parse"))
		return
	}
	var req payload.ConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.RegisterConversation(r.Context(), memberId, req.Id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) deregisterConversation(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse member id from raw string",
			"err", err)
		handleError(w, errors.New("fail to parse"))
		return
	}
	var req payload.ConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.DeregisterConversation(r.Context(), memberId, req.Id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) getTurn(w http.ResponseWriter, _ *http.Request) {
	result := c.service.GenerateTurn()
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) scheduleNotification(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse member id",
			"err", err)
		handleError(w, errors.New("incorrect body"))
	}
	var req payload.ConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("fail to parse body",
			"err", err)
		handleError(w, errors.New("incorrect body"))
	}
	err = c.service.ScheduleNotification(r.Context(), memberId, req.Id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) cancelNotification(w http.ResponseWriter, r *http.Request) {
	memberId, err := uuid.Parse(r.Header.Get("X-User-Id"))
	if err != nil {
		slog.Error("fail to parse member id",
			"err", err)
		handleError(w, errors.New("incorrect body"))
	}
	var req payload.ConversationRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("fail to parse body",
			"err", err)
		handleError(w, errors.New("incorrect body"))
	}
	err = c.service.CancelNotification(r.Context(), memberId, req.Id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) reportConversation(w http.ResponseWriter, r *http.Request) {
	var req payload.ConversationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, errors.New("fail to parse"))
		return
	}
	err = c.service.ReportConversation(r.Context(), req.Id)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
