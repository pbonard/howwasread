package offlineconversation.controller;

import offlineconversation.dto.CreateOfflineConversationRequest;
import offlineconversation.dto.JoinOfflineConversationRequest;
import offlineconversation.dto.UpdateOfflineConversationRequest;
import offlineconversation.service.OfflineConversationService;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotNull;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;
import java.util.UUID;

@RestController
@RequestMapping("/offlineconversation")
@RequiredArgsConstructor
public class OfflineConversationController {

  private final OfflineConversationService offlineConversationService;

  @PostMapping("/create")
  public ResponseEntity<?> create(
      @Valid @RequestBody CreateOfflineConversationRequest request,
      @NotNull @RequestHeader("X-User-Id") UUID memberId) {
    Map<String, UUID> response = offlineConversationService.create(request, memberId);
    return ResponseEntity.ok(response);
  }

  @DeleteMapping("/delete")
  public ResponseEntity<?> delete(
      @NotNull @RequestParam UUID conversationId,
      @NotNull @RequestHeader("X-User-Id") UUID memberId) {
    offlineConversationService.delete(conversationId, memberId);
    return ResponseEntity.ok("ok");
  }

  @PutMapping("/update")
  public ResponseEntity<?> update(
      @Valid @RequestBody UpdateOfflineConversationRequest req,
      @NotNull @RequestHeader("X-User-Id") UUID memberId) {
    offlineConversationService.update(req, memberId);
    return ResponseEntity.ok(req.id().toString());
  }

  @PatchMapping("/join")
  public ResponseEntity<?> join(
      @Valid @RequestBody JoinOfflineConversationRequest request,
      @NotNull @RequestHeader("X-User-Id") UUID memberId
  ) {
    offlineConversationService.join(
        request.conversationId(),
        memberId
    );
    return ResponseEntity.ok("ok");
  }

  @PatchMapping("/quit")
  public ResponseEntity<?> quit(
      @Valid @RequestBody JoinOfflineConversationRequest request,
      @NotNull @RequestHeader("X-User-Id") UUID memberId
  ) {
    offlineConversationService.quit(
        request.conversationId(),
        memberId
    );
    return ResponseEntity.ok("ok");
  }

  @GetMapping("/detail")
  public ResponseEntity<?> detail(
      @NotNull @RequestParam UUID conversationId,
      @NotNull @RequestHeader("X-User-Id") UUID memberId
  ) {
    var response = offlineConversationService.detail(conversationId, memberId);
    return ResponseEntity.ok(response);
  }

  @PostMapping("/report")
  public ResponseEntity<?> report(
      @Valid @RequestBody JoinOfflineConversationRequest request,
      @NotNull @RequestHeader("X-User-Id") UUID memberId
  ) {
    offlineConversationService.report(request.conversationId());
    return ResponseEntity.ok("ok");
  }
}
