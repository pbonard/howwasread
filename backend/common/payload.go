package common

import (
	"encoding/json"

	"github.com/google/uuid"
)

type OnlineConversationSignal struct {
	FromId []byte          `json:"fromId"`
	ToIds  [][]byte        `json:"toIds"`
	Signal json.RawMessage `json:"signal,omitempty"`
}

type ChatMessage struct {
	Id          []byte   `json:"id"`
	FromId      []byte   `json:"fromId"`
	ToIdType    string   `json:"toIdType"`
	ToId        []byte   `json:"toId"`
	ContentType string   `json:"contentType"`
	Contents    []string `json:"contents"`
}

type PreparedMessage struct {
	NotificationId uint8    `json:"notificationId,omitempty"`
	Id             []byte   `json:"id"`
	ToIds          [][]byte `json:"toIds"`
	RoomId         []byte   `json:"roomId"`
	FromId         []byte   `json:"fromId"`
	ContentType    string   `json:"contentType"`
	Contents       []string `json:"contents"`
}

type NotificationMessage struct {
	TokenMap map[string]uuid.UUID `json:"tokenMap"`
	Title    string               `json:"title,omitempty"`
	SubTitle string               `json:"subtitle"`
	Text     string               `json:"text"`
	ImageURL string               `json:"imageURL,omitempty"`
}

type NotificationScheduling struct {
	ScheduledTime int64          `json:"scheduledTime,omitempty"`
	PartitionId   uuid.UUID      `json:"partitionId"`
	KeyId         uuid.UUID      `json:"keyId"`
	Contents      map[int]string `json:"contents"`
	Type          string         `json:"type,omitempty"`
}

type NotificationScheduled struct {
	PartitionId    uuid.UUID                    `json:"partitionId"`
	PartitionType  string                       `json:"partitionType"`
	ScheduledTime  int64                        `json:"scheduledTime"`
	SharedContents map[int]string               `json:"sharedContents"`
	Notifications  map[uuid.UUID]map[int]string `json:"notifications"`
}

// RetryEvent is sent to "exponential-backoff-retry", the job re-sends Value to Topic with Key and Headers
// plus a "partitionId" header holding PartitionId. A follow-up failure only needs PartitionId and Reason,
// []byte fields are base64 in json to match byte[] of the job
type RetryEvent struct {
	PartitionId string            `json:"partitionId"` // "<group>:<topic>:<partition>:<offset>" of the first failed record
	Reason      string            `json:"reason"`
	Backoff     int64             `json:"backoff,omitempty"`
	Multiplier  int64             `json:"multiplier,omitempty"`
	Cap         int64             `json:"cap,omitempty"`
	MaxFailure  int               `json:"maxFailure,omitempty"`
	Topic       string            `json:"topic,omitempty"`
	Key         []byte            `json:"key,omitempty"`
	Headers     map[string][]byte `json:"headers,omitempty"` // original headers without "partitionId"
	Value       []byte            `json:"value,omitempty"`
}

type ConversationRequest struct {
	Id uuid.UUID `json:"id"`
}
