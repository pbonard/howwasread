package offlineconversation.service;

import offlineconversation.client.ModerationClient;
import offlineconversation.component.OutboxPublisher;
import offlineconversation.config.KafkaConfig;
import offlineconversation.dto.ReportContents;
import offlineconversation.dto.Verdict;
import offlineconversation.projection.ReportTargetProjection;
import offlineconversation.repository.OfflineConversationModeratorRepository;
import offlineconversation.repository.OfflineConversationParticipantRepository;
import offlineconversation.repository.OfflineConversationRepository;
import offlineconversation.util.UUIDUtil;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.kafka.support.SendResult;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;
import org.springframework.transaction.support.TransactionTemplate;
import tools.jackson.databind.json.JsonMapper;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class OfflineConversationServiceReportTest {

  @Mock
  private OfflineConversationRepository offlineConversationRepository;
  @Mock
  private OfflineConversationParticipantRepository offlineConversationParticipantRepository;
  @Mock
  private OfflineConversationModeratorRepository offlineConversationModeratorRepository;
  @Mock
  private OutboxPublisher outboxPublisher;
  @Mock
  private ModerationClient moderationClient;
  @Mock
  private KafkaTemplate<byte[], String> kafkaTemplate;
  @Mock
  private PlatformTransactionManager transactionManager;

  private OfflineConversationService offlineConversationService;

  private final UUID conversationId = UUID.randomUUID();
  private final TransactionStatus txStatus = new SimpleTransactionStatus();

  @BeforeEach
  void setUp() {
    lenient().when(transactionManager.getTransaction(any())).thenReturn(txStatus);
    offlineConversationService = new OfflineConversationService(offlineConversationRepository,
        offlineConversationParticipantRepository, offlineConversationModeratorRepository, outboxPublisher,
        moderationClient, kafkaTemplate, new TransactionTemplate(transactionManager), JsonMapper.builder().build());
  }

  @Test
  void report_keysByConversationWithoutValue() {
    when(kafkaTemplate.send(eq(KafkaConfig.OFFLINE_CONVERSATION_TOPIC), eq(UUIDUtil.uuidToBytes(conversationId)), isNull()))
        .thenReturn(CompletableFuture.completedFuture(mock(SendResult.class)));

    offlineConversationService.report(conversationId);

    verify(kafkaTemplate).send(eq(KafkaConfig.OFFLINE_CONVERSATION_TOPIC), eq(UUIDUtil.uuidToBytes(conversationId)), isNull());
  }

  @Test
  void report_failedSendThrows() {
    when(kafkaTemplate.send(anyString(), any(byte[].class), isNull()))
        .thenReturn(CompletableFuture.failedFuture(new RuntimeException("broker down")));

    assertThatThrownBy(() -> offlineConversationService.report(conversationId))
        .isInstanceOf(IllegalStateException.class)
        .hasRootCauseMessage("broker down");
  }

  @Test
  void manageReport_deletedConversationIsDropped() {
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.empty());

    offlineConversationService.manageReport(conversationId);

    verifyNoInteractions(moderationClient, transactionManager);
  }

  @Test
  void manageReport_evaluatedConversationIsSkipped() {
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 1L)));

    offlineConversationService.manageReport(conversationId);

    verifyNoInteractions(moderationClient, transactionManager);
  }

  @Test
  void manageReport_cleanVerdictIsStoredAndConversationKept() {
    Instant updatedAt = Instant.parse("2026-10-01T03:00:00.123456Z");
    Verdict verdict = new Verdict("literary discussion", "none", false);
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(updatedAt, 0L)));
    when(moderationClient.evaluate(contents())).thenReturn(verdict);
    when(moderationClient.model()).thenReturn("qwen3:8b");
    when(offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, updatedAt)).thenReturn(1);

    offlineConversationService.manageReport(conversationId);

    verify(offlineConversationRepository).insertVerdict(eq(conversationId), eq(updatedAt), anyString(), eq("qwen3:8b"), eq(verdict));
    verify(offlineConversationRepository, never()).deleteConversation(any());
    verify(transactionManager).commit(txStatus);
  }

  @Test
  void manageReport_violationIsStoredAndDeletesConversationWithMembers() {
    Verdict verdict = new Verdict("sells a paid course", "advertising", true);
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 0L)));
    when(moderationClient.evaluate(any())).thenReturn(verdict);
    when(moderationClient.model()).thenReturn("qwen3:8b");
    when(offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, null)).thenReturn(1);

    offlineConversationService.manageReport(conversationId);

    verify(offlineConversationRepository).insertVerdict(eq(conversationId), isNull(), anyString(), eq("qwen3:8b"), eq(verdict));
    verify(offlineConversationRepository).deleteConversation(conversationId);
    verify(offlineConversationRepository).deleteParticipants(conversationId);
    verify(offlineConversationRepository).deleteModerators(conversationId);
    verify(transactionManager).commit(txStatus);
  }

  @Test
  void manageReport_failedEvaluationIsRetriedWithoutTransaction() {
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 0L)));
    when(moderationClient.evaluate(any())).thenThrow(new IllegalStateException("ollama timeout"));

    assertThatThrownBy(() -> offlineConversationService.manageReport(conversationId)).hasMessage("ollama timeout");

    verifyNoInteractions(transactionManager);
  }

  @Test
  void manageReport_contentChangedWithCleanVerdictIsRetried() {
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 0L)));
    when(moderationClient.evaluate(any())).thenReturn(new Verdict("", "none", false));
    when(offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, null)).thenReturn(0);

    assertThatThrownBy(() -> offlineConversationService.manageReport(conversationId))
        .hasMessage("conversation contents was okay, but contents changed during report evaluation");

    // nothing is stored, the retryer evaluates the new content
    verify(offlineConversationRepository, never()).insertVerdict(any(), any(), any(), any(), any());
    verify(transactionManager).rollback(txStatus);
  }

  @Test
  void manageReport_contentChangedWithViolationIsStillApplied() {
    Verdict verdict = new Verdict("slur in the description", "hate", true);
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 0L)));
    when(moderationClient.evaluate(any())).thenReturn(verdict);
    when(moderationClient.model()).thenReturn("qwen3:8b");
    when(offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, null)).thenReturn(0);

    offlineConversationService.manageReport(conversationId);

    verify(offlineConversationRepository).insertVerdict(eq(conversationId), isNull(), anyString(), eq("qwen3:8b"), eq(verdict));
    verify(offlineConversationRepository).deleteConversation(conversationId);
    verify(transactionManager).commit(txStatus);
  }

  @Test
  void manageReport_storesTheEvaluatedContents() {
    when(offlineConversationRepository.findReportTarget(conversationId)).thenReturn(Optional.of(target(null, 0L)));
    when(moderationClient.evaluate(any())).thenReturn(new Verdict("ok", "none", false));
    when(moderationClient.model()).thenReturn("qwen3:8b");
    when(offlineConversationRepository.markEvaluatedIfUnchanged(conversationId, null)).thenReturn(1);

    offlineConversationService.manageReport(conversationId);

    ArgumentCaptor<String> json = ArgumentCaptor.forClass(String.class);
    verify(offlineConversationRepository).insertVerdict(eq(conversationId), isNull(), json.capture(), any(), any());
    assertThat(json.getValue()).contains("\"novel\":\"Macbeth\"", "\"location\":\"Seoul\"");
  }

  private ReportContents contents() {
    return ReportContents.builder().novel("Macbeth").writtenBy("shakespeare").description("a reading").location("Seoul").build();
  }

  private ReportTargetProjection target(Instant updatedAt, Long isEvaluated) {
    return new ReportTargetProjection() {
      public String getNovel() { return "Macbeth"; }
      public String getShortStory() { return null; }
      public String getPoem() { return null; }
      public String getPlay() { return null; }
      public String getFilm() { return null; }
      public String getWrittenBy() { return "shakespeare"; }
      public String getDescription() { return "a reading"; }
      public String getLocation() { return "Seoul"; }
      public Instant getUpdatedAt() { return updatedAt; }
      public Long getIsEvaluated() { return isEvaluated; }
    };
  }
}
