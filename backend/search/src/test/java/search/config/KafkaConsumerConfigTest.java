package search.config;

import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.util.backoff.BackOffExecution;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.CompletableFuture;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class KafkaConsumerConfigTest {

  @Test
  void retryBackOffDoublesFourTimesThenStops() {
    BackOffExecution execution = KafkaConsumerConfig.retryBackOff().start();

    assertThat(execution.nextBackOff()).isEqualTo(2000);
    assertThat(execution.nextBackOff()).isEqualTo(4000);
    assertThat(execution.nextBackOff()).isEqualTo(8000);
    assertThat(execution.nextBackOff()).isEqualTo(16000);
    assertThat(execution.nextBackOff()).isEqualTo(BackOffExecution.STOP);
  }

  @Test
  @SuppressWarnings("unchecked")
  void exhaustedRecordIsPublishedToDlqWithItsKeyAndValue() {
    KafkaTemplate<byte[], String> template = mock(KafkaTemplate.class);
    when(template.send(any(ProducerRecord.class))).thenReturn(CompletableFuture.completedFuture(null));
    byte[] key = "conversation-id".getBytes(StandardCharsets.UTF_8);

    KafkaConsumerConfig.deadLetterRecoverer(template)
        .accept(new ConsumerRecord<>("conversation-cdc", 2, 42L, key, "{\"id\":1}"), new RuntimeException("opensearch down"));

    ArgumentCaptor<ProducerRecord<byte[], String>> captor = ArgumentCaptor.forClass(ProducerRecord.class);
    verify(template).send(captor.capture());
    ProducerRecord<byte[], String> sent = captor.getValue();
    assertThat(sent.topic()).isEqualTo(KafkaConsumerConfig.DLQ_TOPIC);
    assertThat(sent.key()).isEqualTo(key);
    assertThat(sent.value()).isEqualTo("{\"id\":1}");
    assertThat(new String(sent.headers().lastHeader("kafka_dlt-original-topic").value(), StandardCharsets.UTF_8))
        .isEqualTo("conversation-cdc");
  }
}
