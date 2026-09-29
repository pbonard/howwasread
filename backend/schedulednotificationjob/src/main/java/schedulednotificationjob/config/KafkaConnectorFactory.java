package schedulednotificationjob.config;

import schedulednotificationjob.dto.IncomingNotificationEvent;
import schedulednotificationjob.dto.OutgoingNotificationEvent;
import schedulednotificationjob.schema.IncomingNotificationEventDeserializationSchema;
import schedulednotificationjob.schema.OutgoingNotificationEventSerializationSchema;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Properties;

import org.apache.flink.connector.kafka.sink.KafkaRecordSerializationSchema;
import org.apache.flink.connector.kafka.sink.KafkaSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.kafka.clients.CommonClientConfigs;
import org.apache.kafka.common.config.SslConfigs;
import org.apache.kafka.common.header.internals.RecordHeader;
import org.apache.kafka.common.header.internals.RecordHeaders;

public class KafkaConnectorFactory {

    private static final String BOOTSTRAP_SERVERS =
        System.getenv().getOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092");

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

    public static KafkaSource<IncomingNotificationEvent> createSource(String topic, String groupId) {
        return KafkaSource.<IncomingNotificationEvent>builder()
            .setBootstrapServers(BOOTSTRAP_SERVERS)
            .setTopics(topic)
            .setGroupId(groupId)
            .setStartingOffsets(OffsetsInitializer.earliest())
            .setValueOnlyDeserializer(new IncomingNotificationEventDeserializationSchema())
            .setProperties(buildSslProperties())
            .build();
    }

    public static KafkaSink<OutgoingNotificationEvent> createSink(String topic) {
        var builder = KafkaSink.<OutgoingNotificationEvent>builder()
            .setBootstrapServers(BOOTSTRAP_SERVERS)
            .setRecordSerializer(
                KafkaRecordSerializationSchema.builder()
                    .setTopic(topic)
                    .setHeaderProvider(element -> new RecordHeaders().add(
                        new RecordHeader("taskType",
                            "scheduled-notification".getBytes(StandardCharsets.UTF_8))
                    ))
                    .setValueSerializationSchema(new OutgoingNotificationEventSerializationSchema())
                    .build()
            );

        Properties sslProps = buildSslProperties();
        if (!sslProps.isEmpty()) {
            builder.setKafkaProducerConfig(sslProps);
        }

        return builder.build();
    }
}