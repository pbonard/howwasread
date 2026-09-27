package test

import (
	"backend/common/mocks"
	"backend/common/payload"
	"backend/common/proto"
	"backend/messagerelay/internal/service"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const serverIP = "10.0.0.1"

var (
	msgId   = uuid.New()
	fromId  = uuid.New()
	roomId  = uuid.New()
	online  = uuid.New()
	offline = uuid.New()
)

type deps struct {
	repo     *MockRepository
	producer *mocks.MockProducer
	relay    *MockRelayClient
	// PreparedMessages pushed to the notification topic
	pushed []payload.PreparedMessage
}

func newService(t *testing.T) (service.Service, *deps) {
	d := &deps{repo: NewMockRepository(t), producer: mocks.NewMockProducer(t), relay: NewMockRelayClient(t)}
	d.producer.EXPECT().PushMessage("notification", []byte(nil), mock.Anything, []sarama.RecordHeader(nil)).
		Run(func(_ string, _ []byte, value []byte, _ []sarama.RecordHeader) {
			var m payload.PreparedMessage
			require.NoError(t, json.Unmarshal(value, &m))
			d.pushed = append(d.pushed, m)
		}).Return().Maybe()
	return service.NewService(d.repo, d.producer, d.relay), d
}

func ips(d *deps, id uuid.UUID, ips ...string) {
	d.repo.EXPECT().GetServerIPs(mock.Anything, string(id[:])).Return(ips, nil).Maybe()
}

func relay(s service.Service, contentType string, toIds ...uuid.UUID) {
	var ids [][]byte
	for _, id := range toIds {
		ids = append(ids, id[:])
	}
	s.RelayMessage(context.Background(), msgId, ids, roomId, fromId, contentType, []string{"hi"})
}

func TestRelayMessage_onlineReceiverIsRelayedToItsServer(t *testing.T) {
	s, d := newService(t)
	ips(d, online, serverIP)
	d.relay.EXPECT().Do(mock.Anything, serverIP, mock.MatchedBy(func(r *proto.RelayMessagingRequest) bool {
		return assert.ObjectsAreEqual([][]byte{online[:]}, r.ToIds) && r.ContentType == "text"
	})).Return(nil, nil)

	relay(s, "text", online)

	assert.Empty(t, d.pushed)
}

func TestRelayMessage_offlineReceiverGetsAPushNotification(t *testing.T) {
	s, d := newService(t)
	ips(d, offline)

	relay(s, "text", offline)

	require.Len(t, d.pushed, 1)
	assert.Equal(t, uint8(0), d.pushed[0].NotificationId)
	assert.Equal(t, [][]byte{offline[:]}, d.pushed[0].ToIds)
	assert.Equal(t, msgId[:], d.pushed[0].Id)
}

func TestRelayMessage_offlineSenderAndRoomEventsAreNotPushed(t *testing.T) {
	for _, contentType := range []string{"create", "participate", "quit"} {
		t.Run(contentType, func(t *testing.T) {
			s, d := newService(t)
			ips(d, offline)

			relay(s, contentType, offline)

			assert.Empty(t, d.pushed)
		})
	}
	t.Run("sender", func(t *testing.T) {
		s, d := newService(t)
		ips(d, fromId)

		relay(s, "text", fromId)

		assert.Empty(t, d.pushed)
	})
}

func TestRelayMessage_failedRelayFallsBackToPushAndForgetsTheServer(t *testing.T) {
	s, d := newService(t)
	ips(d, online, serverIP)
	d.relay.EXPECT().Do(mock.Anything, serverIP, mock.Anything).Return(nil, errors.New("unavailable"))
	d.repo.EXPECT().RemoveServerIP(mock.Anything, string(online[:]), serverIP).Return(nil)

	relay(s, "text", online)

	require.Len(t, d.pushed, 1)
	assert.Equal(t, uint8(1), d.pushed[0].NotificationId)
	assert.Equal(t, [][]byte{online[:]}, d.pushed[0].ToIds)
}

func TestRelayMessage_serverReportsReceiversItCouldNotDeliverTo(t *testing.T) {
	s, d := newService(t)
	ips(d, online, serverIP)
	d.relay.EXPECT().Do(mock.Anything, serverIP, mock.Anything).Return([][]byte{online[:]}, nil)
	d.repo.EXPECT().RemoveServerIP(mock.Anything, string(online[:]), serverIP).Return(nil)

	relay(s, "text", online)

	require.Len(t, d.pushed, 1)
	assert.Equal(t, uint8(1), d.pushed[0].NotificationId)
	assert.Equal(t, [][]byte{online[:]}, d.pushed[0].ToIds)
}

func TestRelayMessage_movedMemberKeepsItsNewServer(t *testing.T) {
	// the member reconnected to another server meanwhile, its ip is not removed
	s, d := newService(t)
	d.repo.EXPECT().GetServerIPs(mock.Anything, string(online[:])).Return([]string{serverIP}, nil).Once()
	d.repo.EXPECT().GetServerIPs(mock.Anything, string(online[:])).Return([]string{"10.0.0.2"}, nil).Once()
	d.relay.EXPECT().Do(mock.Anything, serverIP, mock.Anything).Return(nil, errors.New("unavailable"))

	relay(s, "text", online)

	require.Len(t, d.pushed, 1)
}
