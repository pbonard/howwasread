package offlineconversation.consumer;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import offlineconversation.config.KafkaConfig;
import offlineconversation.dto.RetryEvent;
import offlineconversation.service.OfflineConversationService;
import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.apache.kafka.common.header.Header;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;
import tools.jackson.databind.ObjectMapper;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ThreadLocalRandom;

@Slf4j
@Component
@RequiredArgsConstructor
public class ReportConsumer {

  private static final String PARTITION_ID_HEADER = "partitionId";
  private static final String PARTITION_ID_PREFIX = KafkaConfig.GROUP_ID + ":";

  private final OfflineConversationService offlineConversationService;
  private final KafkaTemplate<byte[], String> kafkaTemplate;
  private final ObjectMapper objectMapper;

  @KafkaListener(topics = KafkaConfig.OFFLINE_CONVERSATION_TOPIC, groupId = KafkaConfig.GROUP_ID)
  public void consume(ConsumerRecord<byte[], String> record) throws ExecutionException, InterruptedException {
    String partitionId = partitionId(record);
    // a retry re-sent for another consumer group of this topic
    if (partitionId != null && !partitionId.startsWith(PARTITION_ID_PREFIX)) {
      return;
    }
    if (record.key() == null || record.key().length != 16) {
      log.error("fail to parse report, key length: {}", record.key() == null ? 0 : record.key().length);
      return;
    }
    UUID conversationId = toUuid(record.key());
    try {
      offlineConversationService.manageReport(conversationId);
    } catch (RuntimeException e) {
      log.atWarn()
          .setMessage("fail to manage report, hand it to the retryer")
          .addKeyValue("conversationId", conversationId)
          .setCause(e)
          .log();
      // a failed hand-off throws, the error handler retries the record instead of skipping it
      kafkaTemplate.send(KafkaConfig.RETRY_TOPIC, retryKey(record, partitionId),
          objectMapper.writeValueAsString(retryEvent(record, partitionId, e))).get();
    }
  }

  private static RetryEvent retryEvent(ConsumerRecord<byte[], String> record, String partitionId, Exception e) {
    String reason = String.valueOf(e.getMessage());
    // reuse first failed message's retry id, the job keeps the rest
    if (partitionId != null) {
      return RetryEvent.builder().partitionId(partitionId).reason(reason).build();
    }
    Map<String, byte[]> headers = new HashMap<>();
    for (Header h : record.headers()) {
      if (!PARTITION_ID_HEADER.equals(h.key())) {
        headers.put(h.key(), h.value());
      }
    }
    return RetryEvent.builder()
        .partitionId(firstPartitionId(record))
        .reason(reason)
        .backoff(addJitter(2000))
        .multiplier(2L)
        .cap(15000000L)
        .maxFailure(5)
        .topic(record.topic())
        .key(record.key())
        .headers(headers)
        .value(record.value() == null ? null : record.value().getBytes(StandardCharsets.UTF_8))
        .build();
  }

  // keyed by partition id so the events of one retry stay ordered in the job
  private static byte[] retryKey(ConsumerRecord<byte[], String> record, String partitionId) {
    return (partitionId != null ? partitionId : firstPartitionId(record)).getBytes(StandardCharsets.UTF_8);
  }

  private static String firstPartitionId(ConsumerRecord<byte[], String> record) {
    return "%s:%s:%d:%d".formatted(KafkaConfig.GROUP_ID, record.topic(), record.partition(), record.offset());
  }

  private static String partitionId(ConsumerRecord<byte[], String> record) {
    Header h = record.headers().lastHeader(PARTITION_ID_HEADER);
    return h == null ? null : new String(h.value(), StandardCharsets.UTF_8);
  }

  // ±10% of base, same as common.AddJitter
  private static long addJitter(long base) {
    if (base <= 0) {
      return base;
    }
    double jitter = base * 0.10 * (ThreadLocalRandom.current().nextDouble() * 2.0 - 1.0);
    return base + (long) jitter;
  }

  private static UUID toUuid(byte[] key) {
    ByteBuffer buffer = ByteBuffer.wrap(key);
    return new UUID(buffer.getLong(), buffer.getLong());
  }
}
