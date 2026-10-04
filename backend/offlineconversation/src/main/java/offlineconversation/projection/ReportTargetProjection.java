package offlineconversation.projection;

import java.time.Instant;

public interface ReportTargetProjection {
  String getNovel();
  String getShortStory();
  String getPoem();
  String getPlay();
  String getFilm();
  String getWrittenBy();
  String getDescription();
  String getLocation();
  Instant getUpdatedAt(); // null until the conversation is updated
  // selected as SIGNED, a BOOLEAN column may come back as Boolean or Integer depending on the driver metadata
  Long getIsEvaluated();
}
