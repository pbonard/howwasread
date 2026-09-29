package internal

import (
	"backend/common/producer"
	"backend/messagepersist/internal/consumer"
	"backend/messagepersist/internal/repository"
	"backend/messagepersist/internal/service"
	"log/slog"
	"os"
)

func NewServer() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	r := repository.NewRepository()

	s := service.NewService(r)

	p := producer.NewSyncProducer("message-persist")

	c := consumer.NewConsumer(s, p)

	err := c.GetMessage([]string{"prepared-message"})
	if err != nil {
		panic(err)
	}
}
