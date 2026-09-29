package service

import (
	"backend/common/producer"
	"backend/fcmnotification/internal/client"
	"backend/fcmnotification/internal/repository"
	"context"

	"github.com/google/uuid"
)

type Service interface {
	SendNotification(ctx context.Context, messageId uuid.UUID, notificationId uint8, value []byte) error
}

type service struct {
	producer   producer.SyncProducer
	repository repository.Repository
	fcmClient  client.FCMClient
}

func NewService(r repository.Repository, p producer.SyncProducer, fcmClient client.FCMClient) Service {
	s := service{
		producer:   p,
		repository: r,
		fcmClient:  fcmClient,
	}
	return &s
}
