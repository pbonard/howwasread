package exponentialbackoffretryjob;

import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import org.apache.flink.api.common.functions.OpenContext;
import org.apache.flink.api.common.state.ListState;
import org.apache.flink.api.common.state.ListStateDescriptor;
import org.apache.flink.api.common.state.MapState;
import org.apache.flink.api.common.state.MapStateDescriptor;
import org.apache.flink.api.common.state.StateDescriptor;
import org.apache.flink.api.common.state.StateTtlConfig;
import org.apache.flink.api.common.state.ValueState;
import org.apache.flink.api.common.state.ValueStateDescriptor;
import org.apache.flink.streaming.api.functions.KeyedProcessFunction;
import org.apache.flink.util.Collector;
import exponentialbackoffretryjob.dto.IncomingEvent;
import exponentialbackoffretryjob.dto.OutgoingEvent;

import java.io.Serial;
import java.time.Duration;
import java.util.HashMap;
import java.util.Map;

@Slf4j
public class ExponentialBackoffRetryFunction extends KeyedProcessFunction<String, IncomingEvent, OutgoingEvent> {

  @Serial
  private static final long serialVersionUID = 1L;

  public static final String DLQ_TOPIC = "dlq";

  private static final StateTtlConfig TTL = StateTtlConfig.newBuilder(Duration.ofHours(24))
      .setUpdateType(StateTtlConfig.UpdateType.OnCreateAndWrite)
      .setStateVisibility(StateTtlConfig.StateVisibility.NeverReturnExpired)
      .build();

  private transient ListState<String> reasons;
  private transient ValueState<Long> backoff;
  private transient ValueState<Long> multiplier;
  private transient ValueState<Long> cap;
  private transient ValueState<Integer> currentFailure;
  private transient ValueState<Integer> maxFailure;
  private transient ValueState<String> topic;
  private transient ValueState<byte[]> key;
  private transient MapState<String, byte[]> headers;
  private transient ValueState<byte[]> value;
  private transient ObjectMapper objectMapper;

  @Override
  public void open(OpenContext openContext) throws Exception {
    reasons = getRuntimeContext().getListState(withTtl(new ListStateDescriptor<>("reasons", String.class)));
    backoff = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("backoff", Long.class)));
    multiplier = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("multiplier", Long.class)));
    cap = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("cap", Long.class)));
    currentFailure = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("current-failure", Integer.class)));
    maxFailure = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("max-failure", Integer.class)));
    topic = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("topic", String.class)));
    key = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("key", byte[].class)));
    headers = getRuntimeContext().getMapState(withTtl(new MapStateDescriptor<>("headers", String.class, byte[].class)));
    value = getRuntimeContext().getState(withTtl(new ValueStateDescriptor<>("value", byte[].class)));
    objectMapper = new ObjectMapper();
  }

  private static <D extends StateDescriptor<?, ?>> D withTtl(D descriptor) {
    descriptor.enableTimeToLive(TTL);
    return descriptor;
  }

  @Override
  public void processElement(IncomingEvent event, Context ctx, Collector<OutgoingEvent> out) throws Exception {
    if (topic.value() == null) {
      // a follow-up whose state expired has nothing to re-send
      if (event.getTopic() == null) {
        log.warn("drop retry event without state, partitionId: {}, reason: {}", event.getPartitionId(), event.getReason());
        return;
      }
      topic.update(event.getTopic());
      key.update(event.getKey());
      if (event.getHeaders() != null) {
        headers.putAll(event.getHeaders());
      }
      value.update(event.getValue());
      backoff.update(event.getBackoff());
      multiplier.update(event.getMultiplier());
      cap.update(event.getCap());
      maxFailure.update(event.getMaxFailure());
      currentFailure.update(0);
    }
    reasons.add(event.getReason());
    int c = currentFailure.value() + 1;
    currentFailure.update(c);
    if (c >= maxFailure.value()) {
      out.collect(OutgoingEvent.builder()
          .topic(DLQ_TOPIC)
          .originalTopic(topic.value())
          .partitionId(ctx.getCurrentKey())
          .key(key.value())
          .headers(storedHeaders())
          .rawReasons(objectMapper.writeValueAsBytes(reasons.get()))
          .value(value.value())
          .build());
      clear();
      return;
    }
    // the first retry waits the base backoff, each next one multiplied up to the cap
    long wait = Math.min(backoff.value(), cap.value());
    backoff.update(Math.min(wait * multiplier.value(), cap.value()));
    ctx.timerService().registerProcessingTimeTimer(ctx.timerService().currentProcessingTime() + wait);
  }

  @Override
  public void onTimer(long timestamp, OnTimerContext ctx, Collector<OutgoingEvent> out) throws Exception {
    if (topic.value() == null) {
      return;
    }
    // the state is kept so a follow-up failure of the re-sent record finds it
    out.collect(OutgoingEvent.builder()
        .topic(topic.value())
        .partitionId(ctx.getCurrentKey())
        .key(key.value())
        .headers(storedHeaders())
        .value(value.value())
        .build());
  }

  private Map<String, byte[]> storedHeaders() throws Exception {
    Map<String, byte[]> m = new HashMap<>();
    for (Map.Entry<String, byte[]> e : headers.entries()) {
      m.put(e.getKey(), e.getValue());
    }
    return m;
  }

  private void clear() {
    reasons.clear();
    backoff.clear();
    multiplier.clear();
    cap.clear();
    currentFailure.clear();
    maxFailure.clear();
    topic.clear();
    key.clear();
    headers.clear();
    value.clear();
  }
}
