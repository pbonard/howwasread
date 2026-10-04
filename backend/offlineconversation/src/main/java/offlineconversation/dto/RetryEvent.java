package offlineconversation.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.Builder;

import java.util.Map;

// sent to "exponential-backoff-retry", the job re-sends value to topic with key and headers
// plus a "partitionId" header holding partitionId. A follow-up failure only needs partitionId and reason,
// byte[] fields are base64 in json to match byte[] of the job
@Builder
@JsonInclude(JsonInclude.Include.NON_NULL)
public record RetryEvent(
    String partitionId, // "<group>:<topic>:<partition>:<offset>" of the first failed record
    String reason,
    Long backoff,
    Long multiplier,
    Long cap,
    Integer maxFailure,
    String topic,
    byte[] key,
    Map<String, byte[]> headers, // original headers without "partitionId"
    byte[] value
) {
}
