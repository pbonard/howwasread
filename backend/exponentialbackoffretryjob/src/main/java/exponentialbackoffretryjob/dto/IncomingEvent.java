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
public class IncomingEvent implements Serializable {

  @Serial
  private static final long serialVersionUID = 1L;

  // "<group>:<topic>:<partition>:<offset>" of the first failed record, the state key
  private String partitionId;
  private String reason;
  // set only on the first failure of a record, a follow-up carries partitionId and reason
  private Long backoff;
  private Long multiplier;
  private Long cap;
  private Integer maxFailure;
  private String topic;
  private byte[] key;
  private Map<String, byte[]> headers;
  private byte[] value;
}
