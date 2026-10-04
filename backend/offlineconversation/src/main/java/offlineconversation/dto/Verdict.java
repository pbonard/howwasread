package offlineconversation.dto;

// the moderation result of a reported conversation
public record Verdict(
    String reason,
    String category,
    boolean violation
) {
}
