package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
)

func (s *service) RegisterNotification(ctx context.Context, id uuid.UUID, os, token string) error {
	old, err := s.repository.FindMemberIdByToken(ctx, token)
	switch {
	case errors.Is(err, gocql.ErrNotFound):
		err = s.repository.UpdateMemberIdByToken(ctx, token, gocql.UUID(id))
		if err != nil {
			return err
		}
	case err != nil:
		slog.Error("fail to find member Id by Token", "err", err,
			"token", token)
		return err
	case old != gocql.UUID(id):
		err = s.repository.DeleteNotificationInfoByIdAndToken(ctx, old, token)
		if err != nil {
			return err
		}
		err = s.repository.UpdateMemberIdByToken(ctx, token, gocql.UUID(id))
		if err != nil {
			return err
		}
	}
	err = s.repository.SaveNotificationInfoById(ctx, gocql.UUID(id), os, token)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) getEachTokenMap(ctx context.Context, toIds []uuid.UUID) (apntm map[string]uuid.UUID, fcmtm map[string]uuid.UUID, err0 error) {
	var em sync.Mutex
	var es []error
	var wg sync.WaitGroup
	apntm = make(map[string]uuid.UUID)
	var am sync.Mutex
	fcmtm = make(map[string]uuid.UUID)
	var fm sync.Mutex
	for _, toId := range toIds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, cancel := context.WithCancel(ctx)
			defer cancel()
			result, err := s.repository.FindPushTokensById(ct, gocql.UUID(toId))
			if err != nil {
				em.Lock()
				es = append(es, err)
				em.Unlock()
				return
			}
			for _, d := range result {
				if d.OS == "ios" {
					am.Lock()
					apntm[d.DevicePushToken] = uuid.UUID(d.Id)
					am.Unlock()
					continue
				}
				fm.Lock()
				fcmtm[d.DevicePushToken] = uuid.UUID(d.Id)
				fm.Unlock()
			}
		}()
	}
	wg.Wait()
	err0 = errors.Join(es...)
	if err0 != nil {
		slog.Error("fail to get each token map", "err0", err0)
		return nil, nil, err0
	}
	return apntm, fcmtm, nil
}
