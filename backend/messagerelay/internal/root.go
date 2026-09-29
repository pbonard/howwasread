package internal

import (
	"backend/common/producer"
	"backend/messagerelay/internal/client"
	"backend/messagerelay/internal/consumer"
	"backend/messagerelay/internal/repository"
	"backend/messagerelay/internal/service"
	"log/slog"
	"os"
)

func NewServer() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	r := repository.NewRepository()

	p := producer.NewSyncProducer("message-relay")

	s := service.NewService(r, p, client.NewRelayClient())

	c := consumer.NewConsumer(s, p)

	err := c.GetMessage([]string{"prepared-message"})
	if err != nil {
		panic(err)
	}
}
