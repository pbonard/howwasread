package exponentialbackoffretryjob.schema;

import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import org.apache.flink.api.common.serialization.DeserializationSchema;
import org.apache.flink.api.common.typeinfo.TypeInformation;
import exponentialbackoffretryjob.dto.IncomingEvent;

import java.io.IOException;
import java.io.Serial;

@Slf4j
public class IncomingEventDeserializationSchema implements DeserializationSchema<IncomingEvent> {
  @Serial
  private static final long serialVersionUID = 1L;

  private transient ObjectMapper objectMapper;

  @Override
  public void open(InitializationContext context) {
    objectMapper = new ObjectMapper();
  }

  @Override
  public IncomingEvent deserialize(byte[] message) throws IOException {
    if (message == null || message.length == 0) {
      return null;
    }
    // a malformed event (e.g. the old format) is skipped instead of failing the job on every restart
    try {
      IncomingEvent event = objectMapper.readValue(message, IncomingEvent.class);
      if (event.getPartitionId() == null) {
        log.error("skip retry event without partitionId");
        return null;
      }
      return event;
    } catch (IOException e) {
      log.error("skip malformed retry event: {}", e.getMessage());
      return null;
    }
  }

  @Override
  public boolean isEndOfStream(IncomingEvent nextElement) {
    return false;
  }

  @Override
  public TypeInformation<IncomingEvent> getProducedType() {
    return TypeInformation.of(IncomingEvent.class);
  }
}
