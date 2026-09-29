package offlineconversation.component;

import offlineconversation.domain.Outbox;
import offlineconversation.repository.OutboxRepository;
import offlineconversation.util.UUIDUtil;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.json.JsonMapper;

import java.util.Base64;
import java.util.List;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.inOrder;

@ExtendWith(MockitoExtension.class)
class OutboxPublisherTest {

  @Mock
  private OutboxRepository outboxRepository;

  private final ObjectMapper objectMapper = JsonMapper.builder().build();
  private OutboxPublisher outboxPublisher;

  private final UUID conversationId = UUID.randomUUID();
  private final UUID memberId = UUID.randomUUID();

  @BeforeEach
  void setUp() {
    outboxPublisher = new OutboxPublisher(outboxRepository, objectMapper);
  }

  @Test
  void publishChatMessage_insertsThenDeletesOutboxRow() {
    outboxPublisher.publishChatMessage(conversationId, memberId, "create", List.of("Seoul"));

    ArgumentCaptor<Outbox> captor = ArgumentCaptor.forClass(Outbox.class);
    InOrder order = inOrder(outboxRepository);
    order.verify(outboxRepository).saveAndFlush(captor.capture());
    Outbox outbox = captor.getValue();
    order.verify(outboxRepository).deleteByIdAndConversationId(outbox.getId(), conversationId);

    assertThat(outbox.getConversationId()).isEqualTo(conversationId);
    assertThat(outbox.getTopic()).isEqualTo("chat-message");
    assertThat(outbox.getTaskType()).isNull();
    assertThat(outbox.isNew()).isTrue();
  }

  @Test
  void publishChatMessage_payloadIsChatMessageWithBase64Ids() {
    outboxPublisher.publishChatMessage(conversationId, memberId, "create", List.of("Seoul"));

    ArgumentCaptor<Outbox> captor = ArgumentCaptor.forClass(Outbox.class);
    inOrder(outboxRepository).verify(outboxRepository).saveAndFlush(captor.capture());
    Outbox outbox = captor.getValue();
    JsonNode payload = objectMapper.readTree(outbox.getPayload());

    assertThat(payload.get("id").asString()).isEqualTo(base64(outbox.getId()));
    assertThat(payload.get("fromId").asString()).isEqualTo(base64(memberId));
    assertThat(payload.get("toId").asString()).isEqualTo(base64(conversationId));
    assertThat(payload.get("toIdType").asString()).isEqualTo("group");
    assertThat(payload.get("contentType").asString()).isEqualTo("create");
    assertThat(payload.get("contents").get(0).asString()).isEqualTo("Seoul");
  }

  @Test
  void publishChatMessage_emptyContents() {
    outboxPublisher.publishChatMessage(conversationId, memberId, "quit", List.of());

    ArgumentCaptor<Outbox> captor = ArgumentCaptor.forClass(Outbox.class);
    inOrder(outboxRepository).verify(outboxRepository).saveAndFlush(captor.capture());
    JsonNode payload = objectMapper.readTree(captor.getValue().getPayload());

    assertThat(payload.get("contentType").asString()).isEqualTo("quit");
    assertThat(payload.get("contents").isEmpty()).isTrue();
  }

  private String base64(UUID uuid) {
    return Base64.getEncoder().encodeToString(UUIDUtil.uuidToBytes(uuid));
  }
}
