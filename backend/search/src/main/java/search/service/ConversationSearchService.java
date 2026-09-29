package search.service;

import search.domain.OfflineConversationDocument;
import search.domain.OnlineConversationDocument;
import search.dto.OfflineConversationMapResponse;
import search.dto.OfflineConversationSearchResponse;
import search.dto.OnlineConversationSearchResponse;
import search.repository.OfflineConversationDocumentRepository;
import search.repository.OnlineConversationDocumentRepository;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.elasticsearch.core.SearchHit;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.support.KafkaHeaders;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Service;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.PropertyNamingStrategies;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;

@Slf4j
@Service
public class ConversationSearchService {

  private final OfflineConversationDocumentRepository offlineConversationDocumentRepository;
  private final OnlineConversationDocumentRepository onlineConversationDocumentRepository;
  private final ObjectMapper cdcMapper;

  public ConversationSearchService(OfflineConversationDocumentRepository offlineConversationDocumentRepository,
                                   OnlineConversationDocumentRepository onlineConversationDocumentRepository,
                                   ObjectMapper objectMapper) {
    this.offlineConversationDocumentRepository = offlineConversationDocumentRepository;
    this.onlineConversationDocumentRepository = onlineConversationDocumentRepository;
    this.cdcMapper = objectMapper.rebuild()
        .propertyNamingStrategy(PropertyNamingStrategies.SNAKE_CASE)
        .build();
  }

  @KafkaListener(topics = "conversation-cdc", groupId = "search")
  public void consume(@Payload(required = false) String payload,
                      @Header(KafkaHeaders.RECEIVED_KEY) byte[] key,
                      @Header("taskType") String type) {
    try {
      switch (type) {
        case "offline_conversation" -> {
          if (payload == null) {
            offlineConversationDocumentRepository.deleteById(toUuid(key));
            return;
          }
          offlineConversationDocumentRepository.save(cdcMapper.readValue(payload, OfflineConversationDocument.class));
        }
        case "online_conversation" -> {
          if (payload == null) {
            onlineConversationDocumentRepository.deleteById(toUuid(key));
            return;
          }
          onlineConversationDocumentRepository.save(cdcMapper.readValue(payload, OnlineConversationDocument.class));
        }
        // member tables are not indexed
        default -> {
        }
      }
    } catch (JacksonException | IllegalArgumentException e) {
      log.error("skip malformed cdc message, type: {}, key: {}", type, new String(key, StandardCharsets.UTF_8), e);
    }
  }

  private UUID toUuid(byte[] key) {
    return UUID.fromString(new String(key, StandardCharsets.UTF_8));
  }

  public List<OfflineConversationSearchResponse> searchOfflines(String input, String resolution, List<String> h3Indexes, Instant time, int page) {
    if (resolution.equals("5")) {
      var searchHits = offlineConversationDocumentRepository
          .findByInputAndH3Res5(input, h3Indexes, time.toEpochMilli(), PageRequest.of(page - 1, 5));
      return buildOfflineConversationSearchResponses(searchHits);
    }
    if (resolution.equals("7")) {
      var searchHits = offlineConversationDocumentRepository
          .findByInputAndH3Res7(input, h3Indexes, time.toEpochMilli(), PageRequest.of(page - 1, 5));
      return buildOfflineConversationSearchResponses(searchHits);
    }
    return null;
  }

  public List<OnlineConversationSearchResponse> searchOnlines(String input, Instant time, int page) {
    var searchHits = onlineConversationDocumentRepository
        .findByInput(input, time.toEpochMilli(), PageRequest.of(page - 1, 5));
    return buildOnlineConversationSearchResponses(searchHits);
  }

  public List<OnlineConversationSearchResponse> listOnlines(Instant time, int page) {
    var conversations = onlineConversationDocumentRepository.findByTimeAfter(
        time.minus(2, ChronoUnit.HOURS).toEpochMilli(),
        PageRequest.of(page - 1, 10, Sort.by(Sort.Direction.ASC, "time")));
    List<OnlineConversationSearchResponse> response = new ArrayList<>();
    for (var conversation : conversations) {
      response.add(OnlineConversationSearchResponse.builder()
          .id(conversation.getId())
          .novel(conversation.getNovel())
          .play(conversation.getPlay())
          .poem(conversation.getPoem())
          .shortStory(conversation.getShortStory())
          .film(conversation.getFilm())
          .writtenBy(conversation.getWrittenBy())
          .time(conversation.getTime())
          .build());
    }
    return response;
  }

  public List<OfflineConversationMapResponse> mapOfflines(String resolution, String h3Index, Instant time) {
    List<OfflineConversationDocument> conversations;
    if (resolution.equals("5")) {
      conversations = offlineConversationDocumentRepository.findByH3Res5AndTimeAfter(
          h3Index, time.toEpochMilli(), PageRequest.of(0, 2, Sort.by(Sort.Direction.ASC, "time")));
    } else if (resolution.equals("7")) {
      conversations = offlineConversationDocumentRepository.findByH3Res7AndTimeAfter(
          h3Index, time.toEpochMilli(), PageRequest.of(0, 50, Sort.by(Sort.Direction.ASC, "time")));
    } else {
      return List.of();
    }
    List<OfflineConversationMapResponse> response = new ArrayList<>();
    for (var conversation : conversations) {
      response.add(OfflineConversationMapResponse.builder()
          .id(conversation.getId())
          .writtenBy(conversation.getWrittenBy())
          .lat(conversation.getLatitude())
          .lng(conversation.getLongitude())
          .build());
    }
    return response;
  }

  private List<OnlineConversationSearchResponse> buildOnlineConversationSearchResponses(List<SearchHit<OnlineConversationDocument>> searchHits) {
    List<OnlineConversationSearchResponse> response = new ArrayList<>();
    for (var searchHit : searchHits) {
      var conversation = searchHit.getContent();
      Map<String, List<String>> highlightFields = searchHit.getHighlightFields();
      response.add(OnlineConversationSearchResponse.builder()
          .id(conversation.getId())
          .novel(getHighlightOrOriginal(highlightFields, "novel", conversation.getNovel()))
          .play(getHighlightOrOriginal(highlightFields, "play", conversation.getPlay()))
          .poem(getHighlightOrOriginal(highlightFields, "poem", conversation.getPoem()))
          .shortStory(getHighlightOrOriginal(highlightFields, "shortStory", conversation.getShortStory()))
          .film(getHighlightOrOriginal(highlightFields, "film", conversation.getFilm()))
          .writtenBy(getHighlightOrOriginal(highlightFields, "writtenBy", conversation.getWrittenBy()))
          .time(conversation.getTime())
          .build());
    }
    return response;
  }

  private List<OfflineConversationSearchResponse> buildOfflineConversationSearchResponses(List<SearchHit<OfflineConversationDocument>> searchHits) {
    List<OfflineConversationSearchResponse> response = new ArrayList<>();
    for (var searchHit : searchHits) {
      var conversation = searchHit.getContent();
      Map<String, List<String>> highlightFields = searchHit.getHighlightFields();
      response.add(OfflineConversationSearchResponse.builder()
          .id(conversation.getId())
          .novel(getHighlightOrOriginal(highlightFields, "novel", conversation.getNovel()))
          .play(getHighlightOrOriginal(highlightFields, "play", conversation.getPlay()))
          .poem(getHighlightOrOriginal(highlightFields, "poem", conversation.getPoem()))
          .shortStory(getHighlightOrOriginal(highlightFields, "shortStory", conversation.getShortStory()))
          .film(getHighlightOrOriginal(highlightFields, "film", conversation.getFilm()))
          .writtenBy(getHighlightOrOriginal(highlightFields, "writtenBy", conversation.getWrittenBy()))
          .time(conversation.getTime())
          .lat(conversation.getLatitude())
          .lng(conversation.getLongitude())
          .build());
    }
    return response;
  }

  private String getHighlightOrOriginal(Map<String, List<String>> highlights, String fieldName, String originalValue) {
    return highlights.containsKey(fieldName) ? highlights.get(fieldName).getFirst() : originalValue;
  }
}
