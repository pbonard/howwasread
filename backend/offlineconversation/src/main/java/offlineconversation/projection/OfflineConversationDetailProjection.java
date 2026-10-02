package offlineconversation.projection;

import java.time.Instant;

public interface OfflineConversationDetailProjection {
  String getNovel();
  String getPoem();
  String getShortStory();
  String getPlay();
  String getFilm();
  String getWrittenBy();
  String getDescription();
  Instant getTime();
  Integer getLengthMinutes();
  String getMapsLink();
  String getLocation();
  Instant getUpdatedAt();
  // EXISTS(...) comes back from MySQL as 0 or 1, projections can't convert it to Boolean
  Long getIsModerator();
  Long getIsParticipant();
  Integer getNumberOfParticipants();
}
