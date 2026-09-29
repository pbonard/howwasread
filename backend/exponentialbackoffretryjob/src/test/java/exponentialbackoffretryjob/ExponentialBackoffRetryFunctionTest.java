package exponentialbackoffretryjob;

import com.fasterxml.jackson.databind.ObjectMapper;
import exponentialbackoffretryjob.dto.IncomingEvent;
import exponentialbackoffretryjob.dto.OutgoingEvent;
import org.apache.flink.api.common.typeinfo.Types;
import org.apache.flink.streaming.util.KeyedOneInputStreamOperatorTestHarness;
import org.apache.flink.streaming.util.ProcessFunctionTestHarnesses;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

class ExponentialBackoffRetryFunctionTest {

  private static final String PARTITION_ID = "message-relay:prepared-message:3:1024";
  private static final byte[] KEY = "key".getBytes(StandardCharsets.UTF_8);
  private static final byte[] VALUE = "{\"id\":1}".getBytes(StandardCharsets.UTF_8);
  private static final byte[] TYPE = "scheduled-notification".getBytes(StandardCharsets.UTF_8);

  private KeyedOneInputStreamOperatorTestHarness<String, IncomingEvent, OutgoingEvent> harness;

  @BeforeEach
  void setUp() throws Exception {
    harness = ProcessFunctionTestHarnesses.forKeyedProcessFunction(
        new ExponentialBackoffRetryFunction(), IncomingEvent::getPartitionId, Types.STRING);
    harness.setProcessingTime(0);
  }

  @AfterEach
  void tearDown() throws Exception {
    harness.close();
  }

  private static IncomingEvent first(int maxFailure) {
    return IncomingEvent.builder()
        .partitionId(PARTITION_ID)
        .reason("first")
        .backoff(2000L)
        .multiplier(2L)
        .cap(5000L)
        .maxFailure(maxFailure)
        .topic("prepared-message")
        .key(KEY)
        .headers(Map.of("taskType", TYPE))
        .value(VALUE)
        .build();
  }

  private static IncomingEvent followUp(String reason) {
    return IncomingEvent.builder().partitionId(PARTITION_ID).reason(reason).build();
  }

  @Test
  void firstFailureIsResentAfterTheBaseBackoffWithKeyAndHeaders() throws Exception {
    harness.processElement(first(5), 0);

    harness.setProcessingTime(1999);
    assertThat(harness.extractOutputValues()).isEmpty();

    harness.setProcessingTime(2000);
    List<OutgoingEvent> out = harness.extractOutputValues();
    assertThat(out).hasSize(1);
    OutgoingEvent e = out.getFirst();
    assertThat(e.getTopic()).isEqualTo("prepared-message");
    assertThat(e.getPartitionId()).isEqualTo(PARTITION_ID);
    assertThat(e.getKey()).isEqualTo(KEY);
    assertThat(e.getHeaders()).containsOnlyKeys("taskType");
    assertThat(e.getHeaders().get("taskType")).isEqualTo(TYPE);
    assertThat(e.getValue()).isEqualTo(VALUE);
  }

  @Test
  void backoffIsMultipliedAndCapped() throws Exception {
    harness.processElement(first(10), 0);
    harness.setProcessingTime(2000);

    // second wait is 2000 * 2 = 4000
    harness.processElement(followUp("second"), 2000);
    harness.setProcessingTime(5999);
    assertThat(harness.extractOutputValues()).hasSize(1);
    harness.setProcessingTime(6000);
    assertThat(harness.extractOutputValues()).hasSize(2);

    // third wait would be 8000, capped at 5000
    harness.processElement(followUp("third"), 6000);
    harness.setProcessingTime(10999);
    assertThat(harness.extractOutputValues()).hasSize(2);
    harness.setProcessingTime(11000);
    assertThat(harness.extractOutputValues()).hasSize(3);
  }

  @Test
  void maxFailureGoesToDlqWithAllReasonsAndClearsState() throws Exception {
    harness.processElement(first(3), 0);
    harness.setProcessingTime(2000);
    harness.processElement(followUp("second"), 2000);
    harness.setProcessingTime(6000);

    harness.processElement(followUp("third"), 6000);

    List<OutgoingEvent> out = harness.extractOutputValues();
    assertThat(out).hasSize(3);
    OutgoingEvent dlq = out.getLast();
    assertThat(dlq.getTopic()).isEqualTo(ExponentialBackoffRetryFunction.DLQ_TOPIC);
    assertThat(dlq.getOriginalTopic()).isEqualTo("prepared-message");
    assertThat(dlq.getPartitionId()).isEqualTo(PARTITION_ID);
    assertThat(dlq.getKey()).isEqualTo(KEY);
    assertThat(dlq.getValue()).isEqualTo(VALUE);
    assertThat(new ObjectMapper().readValue(dlq.getRawReasons(), String[].class))
        .containsExactly("first", "second", "third");

    // state is cleared, a later follow-up has nothing to re-send
    harness.processElement(followUp("late"), 7000);
    harness.setProcessingTime(100000);
    assertThat(harness.extractOutputValues()).hasSize(3);
  }

  @Test
  void followUpWithoutStateIsDropped() throws Exception {
    harness.processElement(followUp("orphan"), 0);

    assertThat(harness.numProcessingTimeTimers()).isZero();
    harness.setProcessingTime(100000);
    assertThat(harness.extractOutputValues()).isEmpty();
  }
}
