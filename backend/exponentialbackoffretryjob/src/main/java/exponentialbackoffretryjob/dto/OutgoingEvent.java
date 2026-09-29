package exponentialbackoffretryjob.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serial;
import java.io.Serializable;
import java.util.Map;


@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class OutgoingEvent implements Serializable {

  @Serial
  private static final long serialVersionUID = 1L;

  private String topic;
  private String originalTopic;
  private String partitionId;
  private byte[] key;
  private Map<String, byte[]> headers;
  private byte[] rawReasons;
  private byte[] value;
}
