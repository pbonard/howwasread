package search.config;

import org.apache.kafka.clients.CommonClientConfigs;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.common.TopicPartition;
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
import org.springframework.kafka.listener.DeadLetterPublishingRecoverer;
import org.springframework.kafka.listener.DefaultErrorHandler;
import org.springframework.kafka.support.ExponentialBackOffWithMaxRetries;
import org.springframework.kafka.support.serializer.JacksonJsonDeserializer;
import org.springframework.util.backoff.BackOff;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.HashMap;
import java.util.Map;

@EnableKafka
@Configuration
public class KafkaConsumerConfig {

  static final String DLQ_TOPIC = "dlq";

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

  // shared by the consumer and the dead-letter producer
  private Map<String, Object> securityConfig() throws IOException {
    Map<String, Object> config = new HashMap<>();

    config.put(CommonClientConfigs.SECURITY_PROTOCOL_CONFIG, "SSL");

    if(!kafkaApiKey.isEmpty()){
      config.put(CommonClientConfigs.SECURITY_PROTOCOL_CONFIG, "SASL_SSL");
      config.put(SaslConfigs.SASL_MECHANISM, "PLAIN");
      String jaasTemplate = "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"%s\" password=\"%s\";";
      config.put(SaslConfigs.SASL_JAAS_CONFIG, String.format(jaasTemplate, kafkaApiKey, kafkaApiSecret));
    }

    if(!kafkaUserCertPath.isEmpty()) {
      String userCert = new String(Files.readAllBytes(Paths.get(kafkaUserCertPath)));
      String userKey = new String(Files.readAllBytes(Paths.get(kafkaUserKeyPath)));
      config.put(SslConfigs.SSL_KEYSTORE_TYPE_CONFIG, "PEM");
      config.put(SslConfigs.SSL_KEYSTORE_CERTIFICATE_CHAIN_CONFIG, userCert);
      config.put(SslConfigs.SSL_KEYSTORE_KEY_CONFIG, userKey);
    }

    if(!kafkaCACertPath.isEmpty()) {
      String caCert = new String(Files.readAllBytes(Paths.get(kafkaCACertPath)));
      config.put(SslConfigs.SSL_TRUSTSTORE_TYPE_CONFIG, "PEM");
      config.put(SslConfigs.SSL_TRUSTSTORE_CERTIFICATES_CONFIG, caCert);
    }

    return config;
  }

  @Bean
  public ConsumerFactory<byte[], Object> consumerFactory() throws IOException {
    Map<String, Object> config = securityConfig();

    config.put(ConsumerConfig.BOOTSTRAP_SERVERS_CONFIG, bootstrapServers);
    config.put(ConsumerConfig.GROUP_ID_CONFIG, "search");
    config.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");

    // Deserialization Properties
    config.put(ConsumerConfig.KEY_DESERIALIZER_CLASS_CONFIG, ByteArrayDeserializer.class);
    config.put(ConsumerConfig.VALUE_DESERIALIZER_CLASS_CONFIG, StringDeserializer.class);

    return new DefaultKafkaConsumerFactory<>(config);
  }

  // serializers mirror the consumer's deserializers, so a dead-lettered record is re-published as it arrived
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
  public KafkaTemplate<byte[], String> kafkaTemplate() throws IOException {
    return new KafkaTemplate<>(producerFactory());
  }

  // cdc records of a conversation must stay in order, so a failed record is retried in place (blocking the partition)
  // instead of the exponential-backoff-retry job, which re-sends later and could overwrite a newer version
  @Bean
  public DefaultErrorHandler errorHandler(KafkaTemplate<byte[], String> kafkaTemplate) {
    return new DefaultErrorHandler(deadLetterRecoverer(kafkaTemplate), retryBackOff());
  }

  // 2s, 4s, 8s, 16s then dead letter: the same budget as the go consumers (5 attempts)
  static BackOff retryBackOff() {
    ExponentialBackOffWithMaxRetries backOff = new ExponentialBackOffWithMaxRetries(4);
    backOff.setInitialInterval(2000);
    backOff.setMultiplier(2);
    backOff.setMaxInterval(30000);
    return backOff;
  }

  // the shared dlq, spring adds the kafka_dlt-* headers (original topic, partition, offset, exception)
  static DeadLetterPublishingRecoverer deadLetterRecoverer(KafkaTemplate<byte[], String> kafkaTemplate) {
    return new DeadLetterPublishingRecoverer(kafkaTemplate, (record, ex) -> new TopicPartition(DLQ_TOPIC, -1));
  }

  @Bean
  public ConcurrentKafkaListenerContainerFactory<byte[], Object> kafkaListenerContainerFactory() throws IOException {
    ConcurrentKafkaListenerContainerFactory<byte[], Object> factory = new ConcurrentKafkaListenerContainerFactory<>();
    factory.setConsumerFactory(consumerFactory());
    factory.setCommonErrorHandler(errorHandler(kafkaTemplate()));
    return factory;
  }
}