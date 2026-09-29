package internal

import (
	"backend/common/producer"
	pb "backend/common/proto"
	"backend/onlineconversation/internal/controller"
	"backend/onlineconversation/internal/grpccontroller"
	"backend/onlineconversation/internal/repository"
	"backend/onlineconversation/internal/service"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"
)

func NewServer() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	kp := producer.NewSyncProducer("online-conversation")
	akp := producer.NewAsyncProducer("online-conversation")

	r := repository.NewRepository()

	s := service.NewService(r, kp, akp)

	mux := http.NewServeMux()

	c := controller.NewController(s, mux)

	go func() {
		err := http.ListenAndServe(":8080", mux)
		if err != nil {
			slog.Error("fail to listen and serve http",
				"err", err)
			panic(err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Panic("fail to create tcp listener at port 50051")
	}
	gc := grpccontroller.NewGRPCController(c)
	g := grpc.NewServer()
	pb.RegisterSignalServiceServer(g, gc)
	err = g.Serve(lis)
	if err != nil {
		log.Panicf("fail to serve: %v", err)
	}
}
