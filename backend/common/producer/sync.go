package producer

import (
	"log"
	"log/slog"
	"os"

	"github.com/IBM/sarama"
)

// SyncProducer returns after the broker acks (RequiredAcks = WaitForAll), or with the error
type SyncProducer interface {
	Commit(topic string, key, value []byte, headers []sarama.RecordHeader) error
	Close() error
}

type syncProducer struct {
	producer sarama.SyncProducer
}

// service is the kafka client id with a "-sync" suffix, e.g. "message-relay-sync"
func NewSyncProducer(service string) SyncProducer {
	cfg, err := baseConfig(service + "-sync")
	if err != nil {
		slog.Error("fail to create sync producer config", "err", err)
		panic(err)
	}
	// SyncProducer requires both
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	// no Flush settings: a single message is sent immediately instead of lingering,
	// concurrent callers are still batched while a request is in flight

	sp, err := sarama.NewSyncProducer([]string{os.Getenv("KAFKA_ADDRESS")}, cfg)
	if err != nil {
		slog.Error("fail to create sync producer", "err", err)
		panic(err)
	}
	log.Print("success to create kafka sync producer")
	return &syncProducer{sp}
}

func (p *syncProducer) Commit(topic string, key, value []byte, headers []sarama.RecordHeader) error {
	_, _, err := p.producer.SendMessage(newMessage(topic, key, value, headers))
	if err != nil {
		slog.Error("fail to send message", "err", err, "topic", topic)
		return err
	}
	return nil
}

func (p *syncProducer) Close() error {
	return p.producer.Close()
}
