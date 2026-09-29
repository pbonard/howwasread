package service

import (
	"backend/common/payload"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
)

// SendNotification returns an error only when the notification was not sent, so a retry never sends it twice
func (s *service) SendNotification(
	ctx context.Context,
	messageId uuid.UUID,
	notificationId uint8,
	value []byte) error {
	did, err := s.repository.DidNotification(ctx, string(messageId[:]), string(notificationId))
	if err != nil {
		return err
	}
	if did {
		return nil
	}
	var p payload.NotificationMessage
	err = json.Unmarshal(value, &p)
	if err != nil {
		slog.Error("fail to unmarshal payload value",
			"err", err,
			"value", value)
		return nil
	}
	tokenMap := p.TokenMap
	text := p.Text
	imageURL := p.ImageURL
	var wg sync.WaitGroup
	var em sync.Mutex
	var es []error
	var ts []string
	for t := range tokenMap {
		ts = append(ts, t)
	}
	if p.SubTitle != "" {
		text = fmt.Sprintf("%v: %v", p.SubTitle, text)
	}
	ctxf, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	invalidTokens, err := s.fcmClient.Send(ctxf, ts, p.Title, text, imageURL)
	if err != nil {
		return err
	}
	// sent already: a failed mark or token cleanup is only logged, a retry would send it again
	err = s.repository.MarkNotification(ctx, string(messageId[:]), string(notificationId))
	if err != nil {
		slog.Error("fail to mark notification", "err", err, "messageId", messageId)
	}
	for _, token := range invalidTokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err2 := s.repository.RemoveNotificationInfoByIdAndToken(ctx, gocql.UUID(tokenMap[token]), token)
			if err2 != nil {
				em.Lock()
				es = append(es, err2)
				em.Unlock()
			}
		}()
	}
	wg.Wait()
	err = errors.Join(es...)
	if err != nil {
		slog.Error("fail to remove invalid tokens", "err", err)
	}
	return nil
}
