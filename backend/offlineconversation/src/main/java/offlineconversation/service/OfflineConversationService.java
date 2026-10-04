package offlineconversation.service;

import offlineconversation.client.ModerationClient;
import offlineconversation.component.OutboxPublisher;
import offlineconversation.config.KafkaConfig;
import offlineconversation.domain.ConversationMemberCompositeKey;
import offlineconversation.domain.OfflineConversation;
import offlineconversation.domain.OfflineConversationModerator;
import offlineconversation.domain.OfflineConversationParticipant;
import offlineconversation.dto.*;
import offlineconversation.projection.ReportTargetProjection;
import offlineconversation.repository.OfflineConversationModeratorRepository;
import offlineconversation.repository.OfflineConversationParticipantRepository;
import offlineconversation.repository.OfflineConversationRepository;
import offlineconversation.util.UUIDUtil;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.web.server.ResponseStatusException;
import tools.jackson.databind.ObjectMapper;

import java.util.*;
import java.util.concurrent.ExecutionException;

@Slf4j
@Service
@Transactional(readOnly = true)
@RequiredArgsConstructor
public class OfflineConversationService {

  private final OfflineConversationRepository offlineConversationRepository;
  private final OfflineConversationParticipantRepository offlineConversationParticipantRepository;
  private final OfflineConversationModeratorRepository offlineConversationModeratorRepository;
  private final OutboxPublisher outboxPublisher;
  private final ModerationClient moderationClient;
  private final KafkaTemplate<byte[], String> kafkaTemplate;
  private final TransactionTemplate transactionTemplate;
  private final ObjectMapper objectMapper;

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

  @Transactional(propagation = Propagation.NOT_SUPPORTED)
  public void report(UUID conversationId) {
    try {
      kafkaTemplate.send(KafkaConfig.OFFLINE_CONVERSATION_TOPIC, UUIDUtil.uuidToBytes(conversationId), null).get();
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException("interrupted while sending report", e);
    } catch (ExecutionException e) {
      log.atError()
          .setMessage("fail to send offline conversation report")
          .addKeyValue("conversationId", conversationId)
          .setCause(e.getCause())
          .log();
      throw new IllegalStateException("fail to send report", e.getCause());
    }
  }

  @Transactional(propagation = Propagation.NOT_SUPPORTED)
  public void manageReport(UUID conversationId) {
    var found = offlineConversationRepository.findReportTarget(conversationId);
    if (found.isEmpty()) {
      return;
    }
    ReportTargetProjection target = found.get();
    if (target.getIsEvaluated() == 1) {
      return;
    }
    var contents = ReportContents.builder()
        .novel(target.getNovel())
        .shortStory(target.getShortStory())
        .poem(target.getPoem())
        .play(target.getPlay())
        .film(target.getFilm())
        .writtenBy(target.getWrittenBy())
        .description(target.getDescription())
        .location(target.getLocation())
        .build();

    Verdict verdict = moderationClient.evaluate(contents);

    transactionTemplate.executeWithoutResult(status -> applyVerdict(conversationId, target, contents, verdict));
  }

  private void applyVerdict(UUID conversationId, ReportTargetProjection target, ReportContents contents, Verdict verdict) {
    int marked = offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, target.getUpdatedAt());
    if (marked == 0) {
      log.atInfo()
          .setMessage("conversation changed during report evaluation")
          .addKeyValue("conversationId", conversationId)
          .addKeyValue("violation", verdict.violation())
          .log();
      if (!verdict.violation()) {
        throw new IllegalStateException("conversation contents was okay, but contents changed during report evaluation");
      }
    }
    offlineConversationRepository.insertVerdict(conversationId, target.getUpdatedAt(),
        objectMapper.writeValueAsString(contents), moderationClient.model(), verdict);
    if (verdict.violation()) {
      log.atInfo()
          .setMessage("delete violating conversation")
          .addKeyValue("conversationId", conversationId)
          .addKeyValue("category", verdict.category())
          .log();
      offlineConversationRepository.deleteConversation(conversationId);
      offlineConversationRepository.deleteParticipants(conversationId);
      offlineConversationRepository.deleteModerators(conversationId);
    }
  }
}
