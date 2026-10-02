package offlineconversation.service;

import offlineconversation.component.OutboxPublisher;
import offlineconversation.domain.ConversationMemberCompositeKey;
import offlineconversation.domain.OfflineConversation;
import offlineconversation.domain.OfflineConversationModerator;
import offlineconversation.domain.OfflineConversationParticipant;
import offlineconversation.dto.*;
import offlineconversation.repository.OfflineConversationModeratorRepository;
import offlineconversation.repository.OfflineConversationParticipantRepository;
import offlineconversation.repository.OfflineConversationRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.server.ResponseStatusException;

import java.util.*;

@Slf4j
@Service
@Transactional(readOnly = true)
@RequiredArgsConstructor
public class OfflineConversationService {

  private final OfflineConversationRepository offlineConversationRepository;
  private final OfflineConversationParticipantRepository offlineConversationParticipantRepository;
  private final OfflineConversationModeratorRepository offlineConversationModeratorRepository;
  private final OutboxPublisher outboxPublisher;

  @Transactional
  public Map<String, UUID> create(
      CreateOfflineConversationRequest req,
      UUID memberId
  ) {
    var convo = OfflineConversation.builder()
        .novel(req.novel())
        .poem(req.poem())
        .shortStory(req.shortStory())
        .play(req.play())
        .film(req.film())
        .writtenBy(req.writtenBy())
        .description(req.description())
        .time(req.time())
        .lengthMinutes(req.lengthMinutes())
        .mapsLink(req.mapsLink())
        .location(req.location())
        .latitude(req.lat())
        .longitude(req.lng())
        .city(req.city())
        .h3Res5(req.h3Res5())
        .h3Res7(req.h3Res7())
        .build();
    var conversationId = offlineConversationRepository.save(convo).getId();
    var key = ConversationMemberCompositeKey.builder()
        .conversationId(conversationId)
        .memberId(memberId)
        .build();

    offlineConversationParticipantRepository.save(new OfflineConversationParticipant(key, convo));
    offlineConversationModeratorRepository.save(new OfflineConversationModerator(key, convo));
    outboxPublisher.publishChatMessage(conversationId, memberId, "create", List.of(req.location()));
    return Map.of("id", convo.getId());
  }

  @Transactional
  public void join(UUID conversationId, UUID memberId) {
    var conversationProxy = offlineConversationRepository.getReferenceById(conversationId);
    var key = ConversationMemberCompositeKey.builder()
        .conversationId(conversationId)
        .memberId(memberId)
        .build();
    offlineConversationParticipantRepository.save(
        new OfflineConversationParticipant(key, conversationProxy));
    outboxPublisher.publishChatMessage(conversationId, memberId, "participate", List.of());
  }

  @Transactional
  public void quit(UUID conversationId, UUID memberId) {
    var key = ConversationMemberCompositeKey.builder()
        .conversationId(conversationId)
        .memberId(memberId)
        .build();
    offlineConversationParticipantRepository.deleteById(key);
    outboxPublisher.publishChatMessage(conversationId, memberId, "quit", List.of());
  }

  public OfflineConversationDetailResponse detail(UUID conversationId, UUID memberId) {
    var convo = offlineConversationRepository.findDetail(conversationId, memberId)
        .orElseThrow(() -> new ResponseStatusException(
            HttpStatus.NOT_FOUND,
            "Conversation not found"
        ));
    return OfflineConversationDetailResponse.builder()
        .novel(convo.getNovel())
        .poem(convo.getPoem())
        .shortStory(convo.getShortStory())
        .play(convo.getPlay())
        .film(convo.getFilm())
        .writtenBy(convo.getWrittenBy())
        .description(convo.getDescription())
        .time(convo.getTime())
        .updatedAt(convo.getUpdatedAt())
        .lengthMinutes(convo.getLengthMinutes())
        .mapsLink(convo.getMapsLink())
        .location(convo.getLocation())
        .isModerator(convo.getIsModerator() == 1)
        .isParticipant(convo.getIsParticipant() == 1)
        .numberOfParticipants(convo.getNumberOfParticipants())
        .build();
  }

  @Transactional
  public void delete(UUID conversationId, UUID memberId) {
    int n = offlineConversationRepository
        .deleteIfModerator(conversationId, memberId);
    if (n == 0) {
      log.atWarn()
          .setMessage("delete offline conversation failed, ui error or api abuse attempt")
          .addKeyValue("conversationId", conversationId)
          .addKeyValue("memberId", memberId)
          .log();
      throw new ResponseStatusException(
          HttpStatus.BAD_REQUEST,
          "can't delete conversation"
      );
    }
  }

  @Transactional
  public void update(UpdateOfflineConversationRequest req, UUID memberId) {
    int n = offlineConversationRepository
        .updateIfModerator(req, memberId);
    if (n == 0) {
      log.atWarn()
          .setMessage("update offline conversation failed, ui error or api abuse attempt")
          .addKeyValue("conversationId", req.id())
          .addKeyValue("memberId", memberId)
          .log();
      throw new ResponseStatusException(
          HttpStatus.BAD_REQUEST,
          "can't update conversation"
      );
    }
  }
}
