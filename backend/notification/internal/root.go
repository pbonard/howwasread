package internal

import (
	"backend/common"
	"backend/common/producer"
	"backend/notification/internal/consumer"
	"backend/notification/internal/controller"
	"backend/notification/internal/repository"
	"backend/notification/internal/service"
	"log/slog"
	"net/http"
	"os"
)

func NewServer() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	r := repository.NewRepository()

	p := producer.NewSyncProducer("notification")

	s := service.NewService(r, p, common.NewCDNClient())

	mux := http.NewServeMux()

	controller.SetController(s, mux)

	go func() {
		err := http.ListenAndServe(":8080", mux)
		if err != nil {
			slog.Error("fail to listen and serve http",
				"err", err)
			panic(err)
		}
	}()

	c := consumer.NewConsumer(s, p)

	err := c.GetMessage([]string{"notification"})
	if err != nil {
		panic(err)
	}
}
