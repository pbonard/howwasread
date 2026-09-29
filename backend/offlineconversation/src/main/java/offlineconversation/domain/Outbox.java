package offlineconversation.domain;

import jakarta.persistence.*;
import lombok.*;
import org.springframework.data.domain.Persistable;

import java.util.UUID;

@Entity
@Builder
@Getter
@NoArgsConstructor
@AllArgsConstructor
public class Outbox implements Persistable<UUID> {
  @Id
  private UUID id;

  @Column(name = "conversation_id")
  private UUID conversationId;

  private String topic;

  // sent as the "taskType" kafka header, null sends no header
  @Column(name = "task_type")
  private String taskType;

  @Column(columnDefinition = "TEXT")
  private String payload;

  // always a new row, so save() persists instead of merging with a select by id(always insert without checking update feasibility)
  @Override
  public boolean isNew() {
    return true;
  }
}
