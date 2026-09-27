package test

import (
	"backend/common/proto"
	"backend/signalrelay/internal/client"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// conversationServer is an in-memory conversation server, it counts dials and answers with the next error
type conversationServer struct {
	proto.UnimplementedSignalServiceServer
	listener *bufconn.Listener
	dials    atomic.Int32

	mu   sync.Mutex
	next error // returned by the next call
}

func (c *conversationServer) failNext(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next = err
}

func (c *conversationServer) RelaySignal(context.Context, *proto.RelaySignalRequest) (*proto.RelaySignalResponse, error) {
	c.mu.Lock()
	err := c.next
	c.next = nil
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &proto.RelaySignalResponse{}, nil
}

func startConversationServer(t *testing.T) (*conversationServer, client.RelayClient) {
	c := &conversationServer{listener: bufconn.Listen(1 << 20)}
	server := grpc.NewServer()
	proto.RegisterSignalServiceServer(server, c)
	go func() { _ = server.Serve(c.listener) }()
	t.Cleanup(server.Stop)

	relay := client.NewRelayClient(grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		c.dials.Add(1)
		return c.listener.DialContext(ctx)
	}))
	return c, relay
}

func TestSignalClient_reusesTheConnectionPerServer(t *testing.T) {
	server, relay := startConversationServer(t)

	for range 3 {
		require.NoError(t, relay.Do(context.Background(), serverIP, &proto.RelaySignalRequest{}))
	}

	assert.Equal(t, int32(1), server.dials.Load())
}

func TestSignalClient_unavailableServerDropsTheConnection(t *testing.T) {
	server, relay := startConversationServer(t)
	server.failNext(status.Error(codes.Unavailable, "restarting"))

	err := relay.Do(context.Background(), serverIP, &proto.RelaySignalRequest{})
	require.ErrorIs(t, err, client.ErrServerUnavailable)
	require.NoError(t, relay.Do(context.Background(), serverIP, &proto.RelaySignalRequest{}))

	assert.Equal(t, int32(2), server.dials.Load())
}

func TestSignalClient_otherErrorsKeepTheConnection(t *testing.T) {
	server, relay := startConversationServer(t)
	server.failNext(status.Error(codes.Internal, "bug"))

	err := relay.Do(context.Background(), serverIP, &proto.RelaySignalRequest{})
	require.Error(t, err)
	assert.NotErrorIs(t, err, client.ErrServerUnavailable)
	require.NoError(t, relay.Do(context.Background(), serverIP, &proto.RelaySignalRequest{}))

	assert.Equal(t, int32(1), server.dials.Load())
}
