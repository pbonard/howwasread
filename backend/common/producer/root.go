package producer

import (
	"backend/common"
	"os"
	"time"

	"github.com/IBM/sarama"
)

// clientId names the application (not the instance), so broker metrics and quotas follow a service across restarts
func baseConfig(clientId string) (*sarama.Config, error) {
	cfg := sarama.NewConfig()

	tlsConfig, err1 := common.CreateTlSConfig(os.Getenv("KAFKA_USER_CERT_PATH"), os.Getenv("KAFKA_USER_KEY_PATH"), os.Getenv("KAFKA_CA_CERT_PATH"))
	if err1 != nil {
		return nil, err1
	}
	cfg.Net.TLS.Config = tlsConfig
	cfg.Net.TLS.Enable = true

	cfg.ClientID = clientId
	if os.Getenv("KAFKA_API_KEY") != "" {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.Version = 1
		cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		cfg.Net.SASL.User = os.Getenv("KAFKA_API_KEY")
		cfg.Net.SASL.Password = os.Getenv("KAFKA_API_SECRET")
		cfg.Net.SASL.Handshake = true
	}

	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Idempotent = false
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Retry.Backoff = time.Millisecond * 300
	cfg.Net.MaxOpenRequests = 5

	// bounded so a blocked SendMessage gives up in about 40-60s instead of minutes
	cfg.Producer.Timeout = time.Second * 5
	cfg.Net.DialTimeout = time.Second * 5
	cfg.Net.ReadTimeout = time.Second * 10
	cfg.Net.WriteTimeout = time.Second * 10

	return cfg, nil
}

func newMessage(topic string, key, value []byte, headers []sarama.RecordHeader) *sarama.ProducerMessage {
	msg := sarama.ProducerMessage{
		Topic:   topic,
		Headers: headers,
		Value:   sarama.ByteEncoder(value),
	}
	if key != nil {
		msg.Key = sarama.ByteEncoder(key)
	}
	return &msg
}
