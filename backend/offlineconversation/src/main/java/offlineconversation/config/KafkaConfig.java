package offlineconversation.config;

import org.apache.kafka.clients.CommonClientConfigs;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.common.config.SaslConfigs;
import org.apache.kafka.common.config.SslConfigs;
import org.apache.kafka.common.serialization.ByteArrayDeserializer;
import org.apache.kafka.common.serialization.ByteArraySerializer;
import org.apache.kafka.common.serialization.StringDeserializer;
import org.apache.kafka.common.serialization.StringSerializer;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.annotation.EnableKafka;
import org.springframework.kafka.config.ConcurrentKafkaListenerContainerFactory;
import org.springframework.kafka.core.ConsumerFactory;
import org.springframework.kafka.core.DefaultKafkaConsumerFactory;
import org.springframework.kafka.core.DefaultKafkaProducerFactory;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.kafka.core.ProducerFactory;
import org.springframework.kafka.listener.DefaultErrorHandler;
import org.springframework.util.backoff.ExponentialBackOff;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.HashMap;
import java.util.Map;

@EnableKafka
@Configuration
public class KafkaConfig {

  public static final String GROUP_ID = "offline-conversation";
  public static final String OFFLINE_CONVERSATION_TOPIC = "offline-conversation";
  public static final String RETRY_TOPIC = "exponential-backoff-retry";

  @Value("${spring.kafka.bootstrap-servers}")
  private String bootstrapServers;

  @Value("${KAFKA_API_KEY:}")
  private String kafkaApiKey;

  @Value("${KAFKA_API_SECRET:}")
  private String kafkaApiSecret;

  @Value("${KAFKA_USER_CERT_PATH:}")
  private String kafkaUserCertPath;

  @Value("${KAFKA_USER_KEY_PATH:}")
  private String kafkaUserKeyPath;

  @Value("${KAFKA_CA_CERT_PATH:}")
  private String kafkaCACertPath;

  // shared by the consumer and the producer
  private Map<String, Object> securityConfig() throws IOException {
    Map<String, Object> config = new HashMap<>();

    config.put(CommonClientConfigs.SECURITY_PROTOCOL_CONFIG, "SSL");

    if (!kafkaApiKey.isEmpty()) {
      config.put(CommonClientConfigs.SECURITY_PROTOCOL_CONFIG, "SASL_SSL");
      config.put(SaslConfigs.SASL_MECHANISM, "PLAIN");
      String jaasTemplate = "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"%s\" password=\"%s\";";
      config.put(SaslConfigs.SASL_JAAS_CONFIG, String.format(jaasTemplate, kafkaApiKey, kafkaApiSecret));
    }

    if (!kafkaUserCertPath.isEmpty()) {
      String userCert = new String(Files.readAllBytes(Paths.get(kafkaUserCertPath)));
      String userKey = new String(Files.readAllBytes(Paths.get(kafkaUserKeyPath)));
      config.put(SslConfigs.SSL_KEYSTORE_TYPE_CONFIG, "PEM");
      config.put(SslConfigs.SSL_KEYSTORE_CERTIFICATE_CHAIN_CONFIG, userCert);
      config.put(SslConfigs.SSL_KEYSTORE_KEY_CONFIG, userKey);
    }

    if (!kafkaCACertPath.isEmpty()) {
      String caCert = new String(Files.readAllBytes(Paths.get(kafkaCACertPath)));
      config.put(SslConfigs.SSL_TRUSTSTORE_TYPE_CONFIG, "PEM");
      config.put(SslConfigs.SSL_TRUSTSTORE_CERTIFICATES_CONFIG, caCert);
    }

    return config;
  }

  @Bean
  public ConsumerFactory<byte[], String> consumerFactory() throws IOException {
    Map<String, Object> config = securityConfig();

    config.put(ConsumerConfig.BOOTSTRAP_SERVERS_CONFIG, bootstrapServers);
    config.put(ConsumerConfig.GROUP_ID_CONFIG, GROUP_ID);
    // a report stored while the worker was down is still consumed
    config.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
    config.put(ConsumerConfig.KEY_DESERIALIZER_CLASS_CONFIG, ByteArrayDeserializer.class);
    config.put(ConsumerConfig.VALUE_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);

    return new DefaultKafkaConsumerFactory<>(config);
  }

  @Bean
  public ProducerFactory<byte[], String> producerFactory() throws IOException {
    Map<String, Object> config = securityConfig();

    config.put(ProducerConfig.BOOTSTRAP_SERVERS_CONFIG, bootstrapServers);
    config.put(ProducerConfig.KEY_SERIALIZER_CLASS_CONFIG, ByteArraySerializer.class);
    config.put(ProducerConfig.VALUE_SERIALIZER_CLASS_CONFIG, StringSerializer.class);
    config.put(ProducerConfig.ACKS_CONFIG, "all");

    return new DefaultKafkaProducerFactory<>(config);
  }

  @Bean
  public KafkaTemplate<byte[], String> kafkaTemplate(ProducerFactory<byte[], String> producerFactory) {
    return new KafkaTemplate<>(producerFactory);
  }

  // a failed report is handed to the exponential-backoff-retry job by the listener itself,
  // this only retries a failed hand-off in place without skipping, like an unmarked offset in the go consumers
  @Bean
  public DefaultErrorHandler errorHandler() {
    ExponentialBackOff backOff = new ExponentialBackOff(2000, 2);
    backOff.setMaxInterval(30000);
    return new DefaultErrorHandler(backOff);
  }

  @Bean
  public ConcurrentKafkaListenerContainerFactory<byte[], String> kafkaListenerContainerFactory(
      ConsumerFactory<byte[], String> consumerFactory, DefaultErrorHandler errorHandler) {
    ConcurrentKafkaListenerContainerFactory<byte[], String> factory = new ConcurrentKafkaListenerContainerFactory<>();
    factory.setConsumerFactory(consumerFactory);
    factory.setCommonErrorHandler(errorHandler);
    return factory;
  }
}
