package search.service;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;
import org.springframework.data.elasticsearch.core.SearchHit;
import search.domain.OfflineConversationDocument;
import search.domain.OnlineConversationDocument;
import search.dto.OfflineConversationMapResponse;
import search.dto.OfflineConversationSearchResponse;
import search.dto.OnlineConversationSearchResponse;
import search.repository.OfflineConversationDocumentRepository;
import search.repository.OnlineConversationDocumentRepository;
import tools.jackson.databind.json.JsonMapper;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class ConversationSearchServiceTest {

  @Mock
  private OfflineConversationDocumentRepository offlineRepository;
  @Mock
  private OnlineConversationDocumentRepository onlineRepository;

  private ConversationSearchService service;

  private final UUID id = UUID.randomUUID();
  private final Instant time = Instant.parse("2026-09-24T03:00:00Z");

  @BeforeEach
  void setUp() {
    service = new ConversationSearchService(offlineRepository, onlineRepository, JsonMapper.builder().build());
  }

  // ---------------------------------------------------------------- consume

  @Test
  void consume_offlineRow_savesMappedDocument() {
    String payload = """
        {"id":"%s","novel":"Hamlet","poem":null,"short_story":"story","play":null,"film":null,
         "written_by":"shakespeare","description":"no spoilers","time":"2026-09-24T03:00:00Z","length_minutes":60,
         "maps_link":"https://maps","location":"Seoul","latitude":37.5,"longitude":127.0,"city":"Seoul",
         "h3_res5":"85283473fffffff","h3_res7":"87283472bffffff","updated_at":null}
        """.formatted(id);

    service.consume(payload, key(id), "offline_conversation");

    ArgumentCaptor<OfflineConversationDocument> captor = ArgumentCaptor.forClass(OfflineConversationDocument.class);
    verify(offlineRepository).save(captor.capture());
    OfflineConversationDocument doc = captor.getValue();
    assertThat(doc.getId()).isEqualTo(id);
    assertThat(doc.getNovel()).isEqualTo("Hamlet");
    assertThat(doc.getShortStory()).isEqualTo("story");
    assertThat(doc.getWrittenBy()).isEqualTo("shakespeare");
    assertThat(doc.getTime()).isEqualTo(time);
    assertThat(doc.getH3Res5()).isEqualTo("85283473fffffff");
    assertThat(doc.getH3Res7()).isEqualTo("87283472bffffff");
    assertThat(doc.getLatitude()).isEqualTo(37.5);
    assertThat(doc.getLongitude()).isEqualTo(127.0);
    verifyNoInteractions(onlineRepository);
  }

  @Test
  void consume_offlineTombstone_deletesById() {
    service.consume(null, key(id), "offline_conversation");

    verify(offlineRepository).deleteById(id);
    verifyNoInteractions(onlineRepository);
  }

  @Test
  void consume_onlineRow_savesMappedDocument() {
    String payload = """
        {"id":"%s","novel":null,"short_story":null,"poem":"Ozymandias","play":null,"film":null,
         "written_by":"shelley","description":null,"capacity":6,"time":"2026-09-24T03:00:00Z","length_minutes":30,
         "current_registrants":1,"updated_at":"2026-09-24T01:00:00Z"}
        """.formatted(id);

    service.consume(payload, key(id), "online_conversation");

    ArgumentCaptor<OnlineConversationDocument> captor = ArgumentCaptor.forClass(OnlineConversationDocument.class);
    verify(onlineRepository).save(captor.capture());
    OnlineConversationDocument doc = captor.getValue();
    assertThat(doc.getId()).isEqualTo(id);
    assertThat(doc.getPoem()).isEqualTo("Ozymandias");
    assertThat(doc.getWrittenBy()).isEqualTo("shelley");
    assertThat(doc.getTime()).isEqualTo(time);
    verifyNoInteractions(offlineRepository);
  }

  @Test
  void consume_onlineTombstone_deletesById() {
    service.consume(null, key(id), "online_conversation");

    verify(onlineRepository).deleteById(id);
    verifyNoInteractions(offlineRepository);
  }

  @Test
  void consume_memberTable_isIgnored() {
    service.consume("{\"conversation_id\":\"%s\",\"member_id\":\"%s\"}".formatted(id, UUID.randomUUID()),
        "{\"conversation_id\":\"x\"}".getBytes(StandardCharsets.UTF_8), "offline_conversation_participant");

    verifyNoInteractions(offlineRepository, onlineRepository);
  }

  @Test
  void consume_malformedJson_isSkipped() {
    service.consume("{not json", key(id), "offline_conversation");

    verifyNoInteractions(offlineRepository, onlineRepository);
  }

  @Test
  void consume_malformedKeyOnTombstone_isSkipped() {
    service.consume(null, "not-a-uuid".getBytes(StandardCharsets.UTF_8), "online_conversation");

    verifyNoInteractions(offlineRepository, onlineRepository);
  }

  // ---------------------------------------------------------------- search / list / map

  @Test
  void searchOfflines_res5_usesHighlightOverOriginal() {
    OfflineConversationDocument doc = offlineDoc();
    SearchHit<OfflineConversationDocument> hit = hit(doc, Map.of("novel", List.of("<em>Ham</em>let")));
    when(offlineRepository.findByInputAndH3Res5("ham", List.of("h3"), time.toEpochMilli(), PageRequest.of(1, 5)))
        .thenReturn(List.of(hit));

    List<OfflineConversationSearchResponse> response = service.searchOfflines("ham", "5", List.of("h3"), time, 2);

    assertThat(response).singleElement().satisfies(r -> {
      assertThat(r.id()).isEqualTo(id);
      assertThat(r.novel()).isEqualTo("<em>Ham</em>let");
      assertThat(r.writtenBy()).isEqualTo("shakespeare");
      assertThat(r.lat()).isEqualTo(37.5);
      assertThat(r.lng()).isEqualTo(127.0);
      assertThat(r.time()).isEqualTo(time);
    });
  }

  @Test
  void searchOfflines_res7_usesRes7Query() {
    SearchHit<OfflineConversationDocument> hit = hit(offlineDoc(), Map.of());
    when(offlineRepository.findByInputAndH3Res7("ham", List.of("h3"), time.toEpochMilli(), PageRequest.of(0, 5)))
        .thenReturn(List.of(hit));

    List<OfflineConversationSearchResponse> response = service.searchOfflines("ham", "7", List.of("h3"), time, 1);

    assertThat(response).singleElement().satisfies(r -> assertThat(r.novel()).isEqualTo("Hamlet"));
  }

  @Test
  void searchOfflines_unknownResolution_returnsNull() {
    assertThat(service.searchOfflines("ham", "9", List.of("h3"), time, 1)).isNull();
    verifyNoInteractions(offlineRepository);
  }

  @Test
  void searchOnlines_mapsHits() {
    OnlineConversationDocument doc = OnlineConversationDocument.builder()
        .id(id).poem("Ozymandias").writtenBy("shelley").time(time).build();
    SearchHit<OnlineConversationDocument> hit = hit(doc, Map.of("poem", List.of("<em>Ozy</em>mandias")));
    when(onlineRepository.findByInput("ozy", time.toEpochMilli(), PageRequest.of(0, 5))).thenReturn(List.of(hit));

    List<OnlineConversationSearchResponse> response = service.searchOnlines("ozy", time, 1);

    assertThat(response).singleElement().satisfies(r -> {
      assertThat(r.poem()).isEqualTo("<em>Ozy</em>mandias");
      assertThat(r.writtenBy()).isEqualTo("shelley");
    });
  }

  @Test
  void listOnlines_queriesFromTwoHoursBeforeSortedByTime() {
    OnlineConversationDocument doc = OnlineConversationDocument.builder()
        .id(id).novel("Hamlet").writtenBy("shakespeare").time(time).build();
    when(onlineRepository.findByTimeAfter(anyLong(), any(Pageable.class))).thenReturn(List.of(doc));

    List<OnlineConversationSearchResponse> response = service.listOnlines(time, 3);

    ArgumentCaptor<Pageable> page = ArgumentCaptor.forClass(Pageable.class);
    verify(onlineRepository).findByTimeAfter(eq(time.minus(2, ChronoUnit.HOURS).toEpochMilli()), page.capture());
    assertThat(page.getValue()).isEqualTo(PageRequest.of(2, 10, Sort.by(Sort.Direction.ASC, "time")));
    assertThat(response).singleElement().satisfies(r -> {
      assertThat(r.id()).isEqualTo(id);
      assertThat(r.novel()).isEqualTo("Hamlet");
    });
  }

  @Test
  void mapOfflines_res5_limitsToTwo() {
    when(offlineRepository.findByH3Res5AndTimeAfter("h3", time.toEpochMilli(),
        PageRequest.of(0, 2, Sort.by(Sort.Direction.ASC, "time")))).thenReturn(List.of(offlineDoc()));

    List<OfflineConversationMapResponse> response = service.mapOfflines("5", "h3", time);

    assertThat(response).singleElement().satisfies(r -> {
      assertThat(r.id()).isEqualTo(id);
      assertThat(r.writtenBy()).isEqualTo("shakespeare");
      assertThat(r.lat()).isEqualTo(37.5);
      assertThat(r.lng()).isEqualTo(127.0);
    });
  }

  @Test
  void mapOfflines_res7_limitsToFifty() {
    when(offlineRepository.findByH3Res7AndTimeAfter("h3", time.toEpochMilli(),
        PageRequest.of(0, 50, Sort.by(Sort.Direction.ASC, "time")))).thenReturn(List.of(offlineDoc()));

    assertThat(service.mapOfflines("7", "h3", time)).hasSize(1);
  }

  @Test
  void mapOfflines_unknownResolution_returnsEmpty() {
    assertThat(service.mapOfflines("9", "h3", time)).isEmpty();
    verifyNoInteractions(offlineRepository);
  }

  // ---------------------------------------------------------------- helpers

  private byte[] key(UUID uuid) {
    return uuid.toString().getBytes(StandardCharsets.UTF_8);
  }

  private OfflineConversationDocument offlineDoc() {
    return OfflineConversationDocument.builder()
        .id(id).novel("Hamlet").writtenBy("shakespeare").time(time).latitude(37.5).longitude(127.0).build();
  }

  @SuppressWarnings("unchecked")
  private <T> SearchHit<T> hit(T content, Map<String, List<String>> highlights) {
    SearchHit<T> hit = mock(SearchHit.class);
    when(hit.getContent()).thenReturn(content);
    when(hit.getHighlightFields()).thenReturn(highlights);
    return hit;
  }
}
