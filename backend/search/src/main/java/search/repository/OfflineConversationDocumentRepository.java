package search.repository;

import search.domain.OfflineConversationDocument;
import org.springframework.data.domain.Pageable;
import org.springframework.data.elasticsearch.annotations.Highlight;
import org.springframework.data.elasticsearch.annotations.HighlightField;
import org.springframework.data.elasticsearch.annotations.HighlightParameters;
import org.springframework.data.elasticsearch.annotations.Query;
import org.springframework.data.elasticsearch.annotations.SourceFilters;
import org.springframework.data.elasticsearch.core.SearchHit;
import org.springframework.data.elasticsearch.repository.ElasticsearchRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.UUID;

@Repository
public interface OfflineConversationDocumentRepository extends ElasticsearchRepository<OfflineConversationDocument, UUID> {

  @SourceFilters(excludes = {"h3Res5", "h3Res7"})
  @Highlight(
      fields = {
          @HighlightField(name = "novel"),
          @HighlightField(name = "poem"),
          @HighlightField(name = "shortStory"),
          @HighlightField(name = "play"),
          @HighlightField(name = "film"),
          @HighlightField(name = "writtenBy")
      },
      parameters = @HighlightParameters(
          preTags = "<em>",
          postTags = "</em>"
      )
  )
  @Query("""
        {
          "bool": {
            "must": [
              {
                "bool": {
                  "should": [
                    {
                      "multi_match": {
                        "query": "?0",
                        "fields": [
                          "novel",
                          "poem",
                          "shortStory",
                          "play",
                          "film",
                          "writtenBy"
                        ],
                        "fuzziness": "AUTO"
                      }
                    },
                    {
                      "multi_match": {
                        "query": "?0",
                        "fields": [
                          "novel.keyword^5",
                          "poem.keyword^5",
                          "shortStory.keyword^5",
                          "play.keyword^5",
                          "film.keyword^5",
                          "writtenBy.keyword^5"
                        ]
                      }
                    }
                  ],
                  "minimum_should_match": 1
                }
              }
            ],
            "filter": [
              {
                "terms": {
                  "h3Res7": ?1
                }
              },
              {
                "range": {
                  "time": {
                    "gt": ?2
                  }
                }
              }
            ]
          }
        }
      """)
  List<SearchHit<OfflineConversationDocument>> findByInputAndH3Res7(
      String input,
      List<String> h3Indexes,
      long time,
      Pageable page
  );

  @SourceFilters(excludes = {"h3Res5", "h3Res7"})
  @Highlight(
      fields = {
          @HighlightField(name = "novel"),
          @HighlightField(name = "poem"),
          @HighlightField(name = "shortStory"),
          @HighlightField(name = "play"),
          @HighlightField(name = "film"),
          @HighlightField(name = "writtenBy")
      },
      parameters = @HighlightParameters(
          preTags = "<em>",
          postTags = "</em>"
      )
  )
  @Query("""
        {
          "bool": {
            "must": [
              {
                "bool": {
                  "should": [
                    {
                      "multi_match": {
                        "query": "?0",
                        "fields": [
                          "novel",
                          "poem",
                          "shortStory",
                          "play",
                          "film",
                          "writtenBy"
                        ],
                        "fuzziness": "AUTO"
                      }
                    },
                    {
                      "multi_match": {
                        "query": "?0",
                        "fields": [
                          "novel.keyword^5",
                          "poem.keyword^5",
                          "shortStory.keyword^5",
                          "play.keyword^5",
                          "film.keyword^5",
                          "writtenBy.keyword^5"
                        ]
                      }
                    }
                  ],
                  "minimum_should_match": 1
                }
              }
            ],
            "filter": [
              {
                "terms": {
                  "h3Res5": ?1
                }
              },
              {
                "range": {
                  "time": {
                    "gt": ?2
                  }
                }
              }
            ]
          }
        }
      """)
  List<SearchHit<OfflineConversationDocument>> findByInputAndH3Res5(
      String input,
      List<String> h3Indexes,
      long time,
      Pageable page
  );

  @SourceFilters(includes = {"writtenBy", "latitude", "longitude"})
  @Query("""
        {
          "bool": {
            "filter": [
              {
                "term": {
                  "h3Res5": "?0"
                }
              },
              {
                "range": {
                  "time": {
                    "gt": ?1
                  }
                }
              }
            ]
          }
        }
      """)
  List<OfflineConversationDocument> findByH3Res5AndTimeAfter(
      String h3Index,
      long time,
      Pageable page
  );

  @SourceFilters(includes = {"writtenBy", "latitude", "longitude"})
  @Query("""
        {
          "bool": {
            "filter": [
              {
                "term": {
                  "h3Res7": "?0"
                }
              },
              {
                "range": {
                  "time": {
                    "gt": ?1
                  }
                }
              }
            ]
          }
        }
      """)
  List<OfflineConversationDocument> findByH3Res7AndTimeAfter(
      String h3Index,
      long time,
      Pageable page
  );
}