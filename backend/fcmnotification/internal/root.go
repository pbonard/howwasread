package internal

import (
	"backend/common/producer"
	"backend/fcmnotification/internal/client"
	"backend/fcmnotification/internal/consumer"
	"backend/fcmnotification/internal/repository"
	"backend/fcmnotification/internal/service"
	"log/slog"
	"os"
)

func NewServer() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	p := producer.NewSyncProducer("fcm-notification")

	r := repository.NewRepository()

	s := service.NewService(r, p, client.NewFCMClient())

	c := consumer.NewConsumer(s, p)

	err := c.GetMessage([]string{"fcm-notification"})
	if err != nil {
		panic(err)
	}
}
