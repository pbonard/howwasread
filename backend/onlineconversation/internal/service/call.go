package service

import (
	"backend/common"
	"backend/onlineconversation/internal/dto"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

func (s *service) GenerateTurn() *dto.GetTurnResponse {
	res := &dto.GetTurnResponse{
		Uris: []string{
			fmt.Sprintf("turn:%s:3478?transport=udp", s.turnRealm),
			fmt.Sprintf("turn:%s:5349?transport=tcp", s.turnRealm),
		},
		Username: fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()),
	}
	mac := hmac.New(sha1.New, []byte(s.turnSecret))
	mac.Write([]byte(res.Username))
	res.Credential = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return res
}

func (s *service) GetParticipantsWithoutMe(ctx context.Context, conversationId string, memberId uuid.UUID) ([]uuid.UUID, error) {
	pidRaws, err := s.repository.FindParticipantIds(ctx, conversationId)
	if err != nil {
		return nil, err
	}
	pids := make([]uuid.UUID, 0, len(pidRaws))
	for _, pidRaw := range pidRaws {
		pid, err1 := uuid.FromBytes([]byte(pidRaw))
		if err1 != nil {
			slog.Error("fail to parse uuid from pidRaw",
				"err", err1,
				"pidRaw", pidRaw)
			return nil, err1
		}
		if memberId == pid {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func (s *service) AddParticipant(ctx context.Context, conversationId string, memberId uuid.UUID) error {
	err := s.repository.AddParticipantId(ctx, conversationId, memberId)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) RemoveParticipant(ctx context.Context, conversationId string, memberId uuid.UUID) error {
	err := s.repository.RemoveParticipantId(ctx, conversationId, memberId)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) SetServerIP(ctx context.Context, memberId uuid.UUID, ip string) error {
	err := s.repository.SetServerIP(ctx, string(memberId[:]), ip)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) RemoveServerIP(ctx context.Context, memberId uuid.UUID) error {
	err := s.repository.RemoveServerIP(ctx, string(memberId[:]))
	if err != nil {
		return err
	}
	return nil
}

func (s *service) PublishConversationSignal(fromId uuid.UUID, toIds [][]byte, signal []byte) error {
	value := common.Marshal(common.OnlineConversationSignal{
		FromId: fromId[:],
		ToIds:  toIds,
		Signal: signal,
	})
	s.asyncProducer.Fire("conversation-signal", nil, value, nil)
	return nil
}
