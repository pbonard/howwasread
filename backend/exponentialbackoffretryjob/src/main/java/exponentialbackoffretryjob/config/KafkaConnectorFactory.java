package exponentialbackoffretryjob.config;

import org.apache.flink.connector.kafka.sink.KafkaRecordSerializationSchema;
import org.apache.flink.connector.kafka.sink.KafkaSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.kafka.clients.CommonClientConfigs;
import org.apache.kafka.common.config.SslConfigs;
import org.apache.kafka.common.header.internals.RecordHeader;
import org.apache.kafka.common.header.internals.RecordHeaders;
import exponentialbackoffretryjob.ExponentialBackoffRetryFunction;
import exponentialbackoffretryjob.dto.IncomingEvent;
import exponentialbackoffretryjob.dto.OutgoingEvent;
import exponentialbackoffretryjob.schema.IncomingEventDeserializationSchema;
import exponentialbackoffretryjob.schema.OutgoingEventSerializationSchema;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Properties;

public class KafkaConnectorFactory {

  private static final String BOOTSTRAP_SERVERS = System.getenv().getOrDefault("KAFKA_BOOTSTRAP_SERVERS",
      "localhost:9092");

  private static Properties buildSslProperties() {
    Properties props = new Properties();

    String userCertPath = System.getenv("KAFKA_USER_CERT_PATH");
    String userKeyPath = System.getenv("KAFKA_USER_KEY_PATH");
    String caCertPath = System.getenv("KAFKA_CA_CERT_PATH");

    if (caCertPath == null || caCertPath.isEmpty()) {
      return props;
    }

    try {
      props.setProperty(CommonClientConfigs.SECURITY_PROTOCOL_CONFIG, "SSL");

      String caCert = Files.readString(Path.of(caCertPath));
      props.setProperty(SslConfigs.SSL_TRUSTSTORE_TYPE_CONFIG, "PEM");
      props.setProperty(SslConfigs.SSL_TRUSTSTORE_CERTIFICATES_CONFIG, caCert);

      if (userCertPath != null && !userCertPath.isEmpty()) {
        String userCert = Files.readString(Path.of(userCertPath));
        String userKey = Files.readString(Path.of(userKeyPath));
        props.setProperty(SslConfigs.SSL_KEYSTORE_TYPE_CONFIG, "PEM");
        props.setProperty(SslConfigs.SSL_KEYSTORE_CERTIFICATE_CHAIN_CONFIG, userCert);
        props.setProperty(SslConfigs.SSL_KEYSTORE_KEY_CONFIG, userKey);
      }
    } catch (IOException e) {
      throw new RuntimeException("Failed to read Kafka SSL certificates", e);
    }

    return props;
  }

  public static KafkaSource<IncomingEvent> createSource(String topic, String groupId) {
    return KafkaSource.<IncomingEvent>builder()
        .setBootstrapServers(BOOTSTRAP_SERVERS)
        .setTopics(topic)
        .setGroupId(groupId)
        .setStartingOffsets(OffsetsInitializer.earliest())
        .setValueOnlyDeserializer(new IncomingEventDeserializationSchema())
        .setProperties(buildSslProperties())
        .build();
  }

  public static KafkaSink<OutgoingEvent> createSink() {
    var builder = KafkaSink.<OutgoingEvent>builder()
        .setBootstrapServers(BOOTSTRAP_SERVERS)
        .setRecordSerializer(
            KafkaRecordSerializationSchema.<OutgoingEvent>builder()
                .setTopicSelector(OutgoingEvent::getTopic)
                // the key is kept so a keyed consumer (e.g. fcm-notification) reads the same record again
                .setKeySerializationSchema(OutgoingEvent::getKey)
                .<OutgoingEvent>setHeaderProvider(event -> {
                  var headers = new RecordHeaders();
                  if (event.getHeaders() != null) {
                    event.getHeaders().forEach((k, v) -> headers.add(new RecordHeader(k, v)));
                  }
                  var partitionId = event.getPartitionId().getBytes(StandardCharsets.UTF_8);
                  if (event.getTopic().equals(ExponentialBackoffRetryFunction.DLQ_TOPIC)) {
                    headers.add(new RecordHeader(
                        "x-original-topic",
                        event.getOriginalTopic().getBytes(StandardCharsets.UTF_8)));
                    headers.add(new RecordHeader("x-partition-id", partitionId));
                    headers.add(new RecordHeader(
                        "reasons",
                        event.getRawReasons()));
                    return headers;
                  }
                  // consumers treat the record as a follow-up only when the id starts with their group
                  headers.add(new RecordHeader("partitionId", partitionId));
                  return headers;
                })
                .setValueSerializationSchema(new OutgoingEventSerializationSchema())
                .build());

    Properties sslProps = buildSslProperties();
    if (!sslProps.isEmpty()) {
      builder.setKafkaProducerConfig(sslProps);
    }

    return builder.build();
  }
}
