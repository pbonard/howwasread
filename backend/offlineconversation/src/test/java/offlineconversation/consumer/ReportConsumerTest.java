package offlineconversation.consumer;

import offlineconversation.config.KafkaConfig;
import offlineconversation.service.OfflineConversationService;
import offlineconversation.util.UUIDUtil;
import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.kafka.support.SendResult;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.json.JsonMapper;

import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class ReportConsumerTest {

  @Mock
  private OfflineConversationService offlineConversationService;
  @Mock
  private KafkaTemplate<byte[], String> kafkaTemplate;

  private final ObjectMapper objectMapper = JsonMapper.builder().build();
  private ReportConsumer consumer;

  private final UUID conversationId = UUID.randomUUID();

  @BeforeEach
  void setUp() {
    consumer = new ReportConsumer(offlineConversationService, kafkaTemplate, objectMapper);
  }

  @Test
  void consume_managesReportOfKeyedConversation() throws Exception {
    consumer.consume(record());

    verify(offlineConversationService).manageReport(conversationId);
    verifyNoInteractions(kafkaTemplate);
  }

  @Test
  void consume_retryOfAnotherGroupIsSkipped() throws Exception {
    var record = record();
    record.headers().add("partitionId", "online-conversation:offline-conversation:0:3".getBytes(StandardCharsets.UTF_8));

    consumer.consume(record);

    verifyNoInteractions(offlineConversationService, kafkaTemplate);
  }

  @Test
  void consume_malformedKeyIsDropped() throws Exception {
    consumer.consume(new ConsumerRecord<>(KafkaConfig.OFFLINE_CONVERSATION_TOPIC, 0, 3, new byte[]{1, 2}, null));

    verifyNoInteractions(offlineConversationService, kafkaTemplate);
  }

  @Test
  void consume_firstFailureSendsFullRetryEvent() throws Exception {
    doThrow(new IllegalStateException("ollama timeout")).when(offlineConversationService).manageReport(conversationId);
    when(kafkaTemplate.send(eq(KafkaConfig.RETRY_TOPIC), any(byte[].class), anyString()))
        .thenReturn(CompletableFuture.completedFuture(mock(SendResult.class)));
    var record = record();
    record.headers().add("taskType", "report".getBytes(StandardCharsets.UTF_8));

    consumer.consume(record);

    String partitionId = "offline-conversation:offline-conversation:1:7";
    JsonNode event = sentRetryEvent(partitionId);
    assertThat(event.get("partitionId").asString()).isEqualTo(partitionId);
    assertThat(event.get("reason").asString()).isEqualTo("ollama timeout");
    assertThat(event.get("backoff").asLong()).isBetween(1800L, 2200L);
    assertThat(event.get("multiplier").asLong()).isEqualTo(2);
    assertThat(event.get("cap").asLong()).isEqualTo(15000000);
    assertThat(event.get("maxFailure").asInt()).isEqualTo(5);
    assertThat(event.get("topic").asString()).isEqualTo(KafkaConfig.OFFLINE_CONVERSATION_TOPIC);
    assertThat(Base64.getDecoder().decode(event.get("key").asString())).isEqualTo(UUIDUtil.uuidToBytes(conversationId));
    assertThat(new String(Base64.getDecoder().decode(event.get("headers").get("taskType").asString()), StandardCharsets.UTF_8))
        .isEqualTo("report");
    assertThat(event.has("value")).as("a report has no value, omitted like go's omitempty").isFalse();
  }

  @Test
  void consume_followUpFailureReusesPartitionIdOnly() throws Exception {
    String partitionId = "offline-conversation:offline-conversation:0:3";
    doThrow(new IllegalStateException("still down")).when(offlineConversationService).manageReport(conversationId);
    when(kafkaTemplate.send(eq(KafkaConfig.RETRY_TOPIC), any(byte[].class), anyString()))
        .thenReturn(CompletableFuture.completedFuture(mock(SendResult.class)));
    var record = record();
    record.headers().add("partitionId", partitionId.getBytes(StandardCharsets.UTF_8));

    consumer.consume(record);

    JsonNode event = sentRetryEvent(partitionId);
    assertThat(event.propertyNames()).containsExactlyInAnyOrder("partitionId", "reason");
    assertThat(event.get("reason").asString()).isEqualTo("still down");
  }

  @Test
  void consume_failedHandOffThrowsSoTheRecordIsRetried() {
    doThrow(new IllegalStateException("ollama timeout")).when(offlineConversationService).manageReport(conversationId);
    when(kafkaTemplate.send(eq(KafkaConfig.RETRY_TOPIC), any(byte[].class), anyString()))
        .thenReturn(CompletableFuture.failedFuture(new RuntimeException("broker down")));

    assertThatThrownBy(() -> consumer.consume(record())).isInstanceOf(ExecutionException.class);
  }

  private ConsumerRecord<byte[], String> record() {
    return new ConsumerRecord<>(KafkaConfig.OFFLINE_CONVERSATION_TOPIC, 1, 7, UUIDUtil.uuidToBytes(conversationId), null);
  }

  private JsonNode sentRetryEvent(String expectedKey) {
    ArgumentCaptor<byte[]> key = ArgumentCaptor.forClass(byte[].class);
    ArgumentCaptor<String> value = ArgumentCaptor.forClass(String.class);
    verify(kafkaTemplate).send(eq(KafkaConfig.RETRY_TOPIC), key.capture(), value.capture());
    // keyed by partition id so the events of one retry stay ordered in the job
    assertThat(new String(key.getValue(), StandardCharsets.UTF_8)).isEqualTo(expectedKey);
    return objectMapper.readTree(value.getValue());
  }
}
