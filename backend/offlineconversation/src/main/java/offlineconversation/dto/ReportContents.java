package offlineconversation.dto;

import lombok.Builder;

// the user-written text of a conversation that moderation judges, stored with its verdict
@Builder
public record ReportContents(
    String novel,
    String shortStory,
    String poem,
    String play,
    String film,
    String writtenBy,
    String description,
    String location
) {
}
