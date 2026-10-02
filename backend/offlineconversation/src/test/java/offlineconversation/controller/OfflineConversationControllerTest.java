package offlineconversation.controller;

import offlineconversation.dto.CreateOfflineConversationRequest;
import offlineconversation.dto.OfflineConversationDetailResponse;
import offlineconversation.dto.UpdateOfflineConversationRequest;
import offlineconversation.service.OfflineConversationService;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.assertj.MockMvcTester;
import org.springframework.web.server.ResponseStatusException;

import java.time.Instant;
import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

// spring.profiles.active is ${PROFILE}, an env variable only set in the cluster
@WebMvcTest(controllers = OfflineConversationController.class, properties = "PROFILE=test")
class OfflineConversationControllerTest {

  private static final String BASE = "/offlineconversation";

  @Autowired
  private MockMvcTester mvc;

  @MockitoBean
  private OfflineConversationService offlineConversationService;

  private final UUID conversationId = UUID.randomUUID();
  private final UUID memberId = UUID.randomUUID();

  @Test
  void create_returnsId() {
    when(offlineConversationService.create(any(CreateOfflineConversationRequest.class), eq(memberId)))
        .thenReturn(Map.of("id", conversationId));

    assertThat(mvc.post().uri(BASE + "/create")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("Seoul")))
        .hasStatusOk()
        .bodyJson().extractingPath("$.id").isEqualTo(conversationId.toString());
  }

  @Test
  void create_withoutUserHeader_isBadRequest() {
    assertThat(mvc.post().uri(BASE + "/create")
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("Seoul")))
        .hasStatus(HttpStatus.BAD_REQUEST);
    verifyNoInteractions(offlineConversationService);
  }

  @Test
  void create_withBlankLocation_isBadRequest() {
    assertThat(mvc.post().uri(BASE + "/create")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("")))
        .hasStatus(HttpStatus.BAD_REQUEST);
    verifyNoInteractions(offlineConversationService);
  }

  @Test
  void delete_returnsOk() {
    assertThat(mvc.delete().uri(BASE + "/delete")
        .param("conversationId", conversationId.toString())
        .header("X-User-Id", memberId.toString()))
        .hasStatusOk()
        .hasBodyTextEqualTo("ok");
    verify(offlineConversationService).delete(conversationId, memberId);
  }

  @Test
  void delete_notModerator_isBadRequest() {
    doThrow(new ResponseStatusException(HttpStatus.BAD_REQUEST, "can't delete conversation"))
        .when(offlineConversationService).delete(conversationId, memberId);

    assertThat(mvc.delete().uri(BASE + "/delete")
        .param("conversationId", conversationId.toString())
        .header("X-User-Id", memberId.toString()))
        .hasStatus(HttpStatus.BAD_REQUEST);
  }

  @Test
  void create_withTooLongTitle_isBadRequest() {
    assertThat(mvc.post().uri(BASE + "/create")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("Seoul", "가".repeat(51), "")))
        .hasStatus(HttpStatus.BAD_REQUEST);
    verifyNoInteractions(offlineConversationService);
  }

  @Test
  void create_withTooLongDescription_isBadRequest() {
    assertThat(mvc.post().uri(BASE + "/create")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("Seoul", "Hamlet", "가".repeat(501))))
        .hasStatus(HttpStatus.BAD_REQUEST);
    verifyNoInteractions(offlineConversationService);
  }

  @Test
  void create_atTheLengthLimits_isOk() {
    when(offlineConversationService.create(any(CreateOfflineConversationRequest.class), eq(memberId)))
        .thenReturn(Map.of("id", conversationId));

    assertThat(mvc.post().uri(BASE + "/create")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(createBody("Seoul", "가".repeat(50), "가".repeat(500))))
        .hasStatusOk();
  }

  @Test
  void update_returnsId() {
    assertThat(mvc.put().uri(BASE + "/update")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content(updateBody()))
        .hasStatusOk()
        .hasBodyTextEqualTo(conversationId.toString());
    verify(offlineConversationService).update(any(UpdateOfflineConversationRequest.class), eq(memberId));
  }

  @Test
  void join_delegatesWithConversationId() {
    assertThat(mvc.patch().uri(BASE + "/join")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content("{\"conversationId\":\"" + conversationId + "\"}"))
        .hasStatusOk()
        .hasBodyTextEqualTo("ok");
    verify(offlineConversationService).join(conversationId, memberId);
  }

  @Test
  void join_withoutConversationId_isBadRequest() {
    assertThat(mvc.patch().uri(BASE + "/join")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content("{}"))
        .hasStatus(HttpStatus.BAD_REQUEST);
    verifyNoInteractions(offlineConversationService);
  }

  @Test
  void quit_delegatesWithConversationId() {
    assertThat(mvc.patch().uri(BASE + "/quit")
        .header("X-User-Id", memberId.toString())
        .contentType(MediaType.APPLICATION_JSON)
        .content("{\"conversationId\":\"" + conversationId + "\"}"))
        .hasStatusOk()
        .hasBodyTextEqualTo("ok");
    verify(offlineConversationService).quit(conversationId, memberId);
  }

  @Test
  void detail_returnsJson() {
    when(offlineConversationService.detail(conversationId, memberId)).thenReturn(
        OfflineConversationDetailResponse.builder()
            .novel("Hamlet")
            .writtenBy("shakespeare")
            .location("Seoul")
            .numberOfParticipants(3)
            .build());

    assertThat(mvc.get().uri(BASE + "/detail")
        .param("conversationId", conversationId.toString())
        .header("X-User-Id", memberId.toString()))
        .hasStatusOk()
        .bodyJson().extractingPath("$.writtenBy").isEqualTo("shakespeare");
  }

  @Test
  void detail_missing_isNotFound() {
    when(offlineConversationService.detail(conversationId, memberId))
        .thenThrow(new ResponseStatusException(HttpStatus.NOT_FOUND, "Conversation not found"));

    assertThat(mvc.get().uri(BASE + "/detail")
        .param("conversationId", conversationId.toString())
        .header("X-User-Id", memberId.toString()))
        .hasStatus(HttpStatus.NOT_FOUND);
  }

  private String createBody(String location) {
    return createBody(location, "Hamlet", "");
  }

  private String createBody(String location, String novel, String description) {
    return """
        {"novel":"%s","writtenBy":"shakespeare","description":"%s","time":"%s","lengthMinutes":60,
         "mapsLink":"https://maps","location":"%s","city":"Seoul","lat":37.5,"lng":127.0,
         "h3Res5":"85283473fffffff","h3Res7":"87283472bffffff"}
        """.formatted(novel, description, Instant.parse("2026-09-24T03:00:00Z"), location);
  }

  private String updateBody() {
    return """
        {"id":"%s","novel":"Hamlet","writtenBy":"shakespeare","time":"2026-09-24T03:00:00Z","lengthMinutes":60,
         "mapsLink":"https://maps","location":"Seoul","city":"Seoul","lat":37.5,"lng":127.0,
         "h3Res5":"85283473fffffff","h3Res7":"87283472bffffff"}
        """.formatted(conversationId);
  }
}
