package test

import (
	"backend/common/proto"
	"backend/messagerelay/internal/client"
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// chatServer is an in-memory chat server, it counts dials and can fail its next call
type chatServer struct {
	proto.UnimplementedMessagingServiceServer
	listener *bufconn.Listener
	dials    atomic.Int32
	fail     atomic.Bool
	lastReq  *proto.RelayMessagingRequest
}

func (c *chatServer) RelayMessaging(_ context.Context, req *proto.RelayMessagingRequest) (*proto.RelayMessagingResponse, error) {
	c.lastReq = req
	if c.fail.Swap(false) {
		return nil, errors.New("server busy")
	}
	return &proto.RelayMessagingResponse{PushToIds: req.ToIds[1:]}, nil
}

func startChatServer(t *testing.T) (*chatServer, client.RelayClient) {
	c := &chatServer{listener: bufconn.Listen(1 << 20)}
	server := grpc.NewServer()
	proto.RegisterMessagingServiceServer(server, c)
	go func() { _ = server.Serve(c.listener) }()
	t.Cleanup(server.Stop)

	relay := client.NewRelayClient(grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		c.dials.Add(1)
		return c.listener.DialContext(ctx)
	}))
	return c, relay
}

func TestRelayClient_relaysAndReturnsUndeliveredReceivers(t *testing.T) {
	server, relay := startChatServer(t)

	undelivered, err := relay.Do(context.Background(), "10.0.0.1",
		&proto.RelayMessagingRequest{ToIds: [][]byte{online[:], offline[:]}, ContentType: "text"})

	require.NoError(t, err)
	assert.Equal(t, [][]byte{offline[:]}, undelivered)
	assert.Equal(t, "text", server.lastReq.ContentType)
}

func TestRelayClient_reusesTheConnectionPerServer(t *testing.T) {
	server, relay := startChatServer(t)
	req := &proto.RelayMessagingRequest{ToIds: [][]byte{online[:]}}

	for range 3 {
		_, err := relay.Do(context.Background(), "10.0.0.1", req)
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), server.dials.Load())
}

func TestRelayClient_failedCallDropsTheConnection(t *testing.T) {
	server, relay := startChatServer(t)
	req := &proto.RelayMessagingRequest{ToIds: [][]byte{online[:]}}
	server.fail.Store(true)

	_, err := relay.Do(context.Background(), "10.0.0.1", req)
	require.Error(t, err)
	_, err = relay.Do(context.Background(), "10.0.0.1", req)
	require.NoError(t, err)

	assert.Equal(t, int32(2), server.dials.Load())
}
