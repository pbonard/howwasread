package offlineconversation.dto;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

import java.time.Instant;

public record CreateOfflineConversationRequest(
    @Size(max = 50) String novel,
    @Size(max = 50) String poem,
    @Size(max = 50) String shortStory,
    @Size(max = 50) String play,
    @Size(max = 50) String film,
    @NotBlank @Size(max = 50) String writtenBy,
    @Size(max = 500) String description,
    @NotNull Instant time,
    @Min(value = 0) int lengthMinutes,
    @NotBlank String mapsLink,
    @NotBlank String location,
    String city,
    @Min(value = -90) @Max(value = 90) double lat,
    @Min(value = -180) @Max(value = 180) double lng,
    @NotBlank String h3Res5,
    @NotBlank String h3Res7
) {
}