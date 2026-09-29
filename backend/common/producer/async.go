package producer

import (
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/IBM/sarama"
)

// AsyncProducer queues the message and returns, failures are only logged (best-effort)
type AsyncProducer interface {
	Fire(topic string, key, value []byte, headers []sarama.RecordHeader)
	Close() error
}

type asyncProducer struct {
	producer sarama.AsyncProducer
}

// service is the kafka client id with a "-async" suffix, e.g. "message-relay-async"
func NewAsyncProducer(service string) AsyncProducer {
	cfg, err := baseConfig(service + "-async")
	if err != nil {
		slog.Error("fail to create async producer config", "err", err)
		panic(err)
	}
	cfg.Producer.Return.Successes = false
	cfg.Producer.Return.Errors = true
	cfg.Producer.Flush.Messages = 100
	cfg.Producer.Flush.Frequency = time.Millisecond * 5

	ap, err := sarama.NewAsyncProducer([]string{os.Getenv("KAFKA_ADDRESS")}, cfg)
	if err != nil {
		slog.Error("fail to create async producer", "err", err)
		panic(err)
	}
	log.Print("success to create kafka async producer")
	p := asyncProducer{ap}

	go p.drainErrorChannel()
	return &p
}

func (p *asyncProducer) Fire(topic string, key, value []byte, headers []sarama.RecordHeader) {
	p.producer.Input() <- newMessage(topic, key, value, headers)
}

func (p *asyncProducer) Close() error {
	return p.producer.Close()
}

func (p *asyncProducer) drainErrorChannel() {
	for err := range p.producer.Errors() {
		slog.Error("fail to produce payload",
			"err", err.Err,
			"topic", err.Msg.Topic,
		)
		//TODO: dlq? or send notification to developer directly
	}
}
