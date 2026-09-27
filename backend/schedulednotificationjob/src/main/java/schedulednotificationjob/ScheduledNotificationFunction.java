package schedulednotificationjob;

import schedulednotificationjob.dto.IncomingNotificationEvent;
import schedulednotificationjob.domain.NotificationElement;
import schedulednotificationjob.dto.OutgoingNotificationEvent;
import org.apache.flink.api.common.functions.OpenContext;
import org.apache.flink.api.common.state.MapState;
import org.apache.flink.api.common.state.MapStateDescriptor;
import org.apache.flink.api.common.state.ValueState;
import org.apache.flink.api.common.state.ValueStateDescriptor;
import org.apache.flink.api.common.typeinfo.TypeHint;
import org.apache.flink.api.common.typeinfo.TypeInformation;
import org.apache.flink.streaming.api.functions.KeyedProcessFunction;
import org.apache.flink.util.Collector;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.Serial;
import java.util.*;

public class ScheduledNotificationFunction extends KeyedProcessFunction<String, IncomingNotificationEvent, OutgoingNotificationEvent> {

  @Serial
  private static final long serialVersionUID = 1L;
  private static final Logger LOG = LoggerFactory.getLogger(ScheduledNotificationFunction.class);

  private transient MapState<String, NotificationElement> elements;
  private transient ValueState<String> partitionType;
  private transient ValueState<Map<Integer, String>> sharedContents;

  @Override
  public void open(OpenContext openContext) throws Exception {
    MapStateDescriptor<String, NotificationElement> eventsDescriptor =
        new MapStateDescriptor<>("elements", String.class, NotificationElement.class);
    elements = getRuntimeContext().getMapState(eventsDescriptor);

    ValueStateDescriptor<String> partitionTypeDescriptor =
        new ValueStateDescriptor<>("partition-type", String.class);
    partitionType = getRuntimeContext().getState(partitionTypeDescriptor);

    ValueStateDescriptor<Map<Integer, String>> sharedContentsDescriptor =
        new ValueStateDescriptor<>("shared-contents", TypeInformation.of(new TypeHint<Map<Integer, String>>() {
        }));
    sharedContents = getRuntimeContext().getState(sharedContentsDescriptor);
  }

  @Override
  public void processElement(IncomingNotificationEvent event, Context ctx, Collector<OutgoingNotificationEvent> out) throws Exception {
    if (event.getType() == null || event.getKeyId() == null) {
      LOG.warn("drop notification event without type or keyId, partitionId: {}", event.getPartitionId());
      return;
    }
    if (event.getType().equals("cancel")) {
      elements.remove(event.getKeyId());
      return;
    }
    if (event.getScheduledTime() == null || event.getContents() == null) {
      LOG.warn("drop notification event without scheduledTime or contents, partitionId: {}", event.getPartitionId());
      return;
    }
    if (partitionType.value() == null) {
      partitionType.update(event.getType());
    }
    if (partitionType.value().equals("online-conversation")) {
      if (!event.getContents().isEmpty()) {
        sharedContents.update(event.getContents());
      }
      if (event.getScheduledTime() != 0) {
        ctx.timerService().registerProcessingTimeTimer(event.getScheduledTime());
      }
      elements.put(event.getKeyId(), null);
      return;
    }
    var element = NotificationElement.builder()
        .scheduledTime(event.getScheduledTime())
        .contents(event.getContents())
        .build();
    elements.put(event.getKeyId(), element);
  }

  @Override
  public void onTimer(long timestamp, KeyedProcessFunction<String, IncomingNotificationEvent, OutgoingNotificationEvent>.OnTimerContext ctx, Collector<OutgoingNotificationEvent> out) throws Exception {
    Map<String, Map<Integer, String>> notifications = new HashMap<>();
    String partitionId = ctx.getCurrentKey();
    var iterator = elements.entries().iterator();
    while (iterator.hasNext()) {
      var next = iterator.next();
      var element = next.getValue();
      if (partitionType.value().equals("online-conversation") || element.getScheduledTime() <= timestamp) {
        notifications.put(next.getKey(),
            element == null ? null : element.getContents());
        iterator.remove();
      }
    }
    if (partitionId != null && !notifications.isEmpty()) {
      var output = OutgoingNotificationEvent.builder()
          .partitionId(partitionId)
          .partitionType(partitionType.value())
          .sharedContents(sharedContents.value())
          .notifications(notifications)
          .scheduledTime(timestamp)
          .build();
      out.collect(output);
      if (partitionType.value().equals("online-conversation")) {
        elements.clear();
        partitionType.clear();
        sharedContents.clear();
      }
    }
  }
}
