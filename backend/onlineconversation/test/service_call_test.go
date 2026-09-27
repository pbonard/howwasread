package test

import (
	"backend/common/mocks"
	"backend/common/payload"
	"backend/onlineconversation/internal/service"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGenerateTurn_credentialIsHmacOfExpiry(t *testing.T) {
	t.Setenv("TURN_SECRET", "secret")
	t.Setenv("TURN_REALM", "turn.example.com")
	before := time.Now().Add(2 * time.Hour).Unix()

	resp := newService(t, NewMockRepository(t)).GenerateTurn()

	// the username is the unix time the credential expires, 2 hours from now
	expires, err := strconv.ParseInt(resp.Username, 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, expires, before)
	assert.LessOrEqual(t, expires, time.Now().Add(2*time.Hour).Unix())
	mac := hmac.New(sha1.New, []byte("secret"))
	mac.Write([]byte(resp.Username))
	assert.Equal(t, base64.StdEncoding.EncodeToString(mac.Sum(nil)), resp.Credential)
	assert.Equal(t, []string{
		"turn:turn.example.com:3478?transport=udp",
		"turn:turn.example.com:5349?transport=tcp",
	}, resp.Uris)
}

func TestGetParticipantsWithoutMe(t *testing.T) {
	other := uuid.New()
	t.Run("filters out the caller", func(t *testing.T) {
		repo := NewMockRepository(t)
		repo.EXPECT().FindParticipantIds(mock.Anything, "room").
			Return([]string{string(memberId[:]), string(other[:])}, nil)

		ids, err := newService(t, repo).GetParticipantsWithoutMe(context.Background(), "room", memberId)

		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{other}, ids)
	})
	t.Run("stored id that is not 16 bytes", func(t *testing.T) {
		repo := NewMockRepository(t)
		repo.EXPECT().FindParticipantIds(mock.Anything, "room").Return([]string{"short"}, nil)

		_, err := newService(t, repo).GetParticipantsWithoutMe(context.Background(), "room", memberId)

		assert.Error(t, err)
	})
}

func TestServerIPAndParticipantPassThrough(t *testing.T) {
	repo := NewMockRepository(t)
	repo.EXPECT().SetServerIP(mock.Anything, string(memberId[:]), "10.0.0.1").Return(nil)
	repo.EXPECT().RemoveServerIP(mock.Anything, string(memberId[:])).Return(nil)
	repo.EXPECT().AddParticipantId(mock.Anything, "room", memberId).Return(nil)
	repo.EXPECT().RemoveParticipantId(mock.Anything, "room", memberId).Return(errDB)
	s := newService(t, repo)

	assert.NoError(t, s.SetServerIP(context.Background(), memberId, "10.0.0.1"))
	assert.NoError(t, s.RemoveServerIP(context.Background(), memberId))
	assert.NoError(t, s.AddParticipant(context.Background(), "room", memberId))
	assert.ErrorIs(t, s.RemoveParticipant(context.Background(), "room", memberId), errDB)
}

func TestPublishConversationSignal(t *testing.T) {
	producer := mocks.NewMockProducer(t)
	other := uuid.New()
	var pushed []byte
	producer.EXPECT().PushMessage("conversation-signal", []byte(nil), mock.Anything, []sarama.RecordHeader(nil)).
		Run(func(_ string, _ []byte, value []byte, _ []sarama.RecordHeader) { pushed = value }).Return()

	err := service.NewService(NewMockRepository(t), producer).
		PublishConversationSignal(memberId, [][]byte{other[:]}, []byte(`{"sdp":"offer"}`))

	require.NoError(t, err)
	var signal payload.OnlineConversationSignal
	require.NoError(t, json.Unmarshal(pushed, &signal))
	assert.Equal(t, memberId[:], signal.FromId)
	assert.Equal(t, [][]byte{other[:]}, signal.ToIds)
	assert.JSONEq(t, `{"sdp":"offer"}`, string(signal.Signal))
}
