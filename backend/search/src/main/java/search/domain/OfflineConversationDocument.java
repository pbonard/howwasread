package search.domain;

import lombok.*;
import org.springframework.data.annotation.Id;
import org.springframework.data.elasticsearch.annotations.*;

import java.time.Instant;
import java.util.UUID;


@Document(indexName = "offline_conversation")
// a single opensearch node can't hold a replica, the index would stay yellow with spring data's default of 1
@Setting(settingPath = "opensearch/standard-setting.json", replicas = 0)
@Getter
@Setter
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class OfflineConversationDocument {
  @Id
  private UUID id;

  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String novel;
  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String poem;
  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String shortStory;
  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String play;
  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String film;
  @MultiField(
      mainField = @Field(type = FieldType.Text, analyzer = "standard_lowercase_analyzer"),
      otherFields = {
          @InnerField(suffix = "keyword", type = FieldType.Keyword, normalizer = "lowercase_normalizer")
      }
  )
  private String writtenBy;

  @Field(type = FieldType.Date)
  private Instant time;

  @Field(type = FieldType.Keyword)
  private String h3Res5;

  @Field(type = FieldType.Keyword)
  private String h3Res7;

  @Field(type = FieldType.Double, index = false)
  private double latitude;

  @Field(type = FieldType.Double, index = false)
  private double longitude;
}
