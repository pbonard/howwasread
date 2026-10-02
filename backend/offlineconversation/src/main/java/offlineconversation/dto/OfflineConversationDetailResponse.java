package offlineconversation.dto;

import lombok.Builder;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

@Builder
public record OfflineConversationDetailResponse(
    String novel,
    String poem,
    String shortStory,
    String play,
    String film,
    String writtenBy,
    String description,
    Instant time,
    int lengthMinutes,
    String mapsLink,
    String location,
    Instant updatedAt,
    boolean isModerator,
    boolean isParticipant,
    int numberOfParticipants,
    List<UUID> moderatorIds
) {
}
