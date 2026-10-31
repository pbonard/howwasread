package search.domain;

import lombok.*;
import org.springframework.data.annotation.Id;
import org.springframework.data.elasticsearch.annotations.*;

import java.time.Instant;
import java.util.UUID;

@Document(indexName = "online_conversation")
// a single opensearch node can't hold a replica, the index would stay yellow with spring data's default of 1
@Setting(settingPath = "opensearch/standard-setting.json", replicas = 0)
@Getter
@Setter
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class OnlineConversationDocument {
  @Id
  private UUID id;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String novel;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String poem;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String shortStory;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String play;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String film;

  @Field(type = FieldType.Search_As_You_Type, analyzer = "standard_lowercase_analyzer")
  private String writtenBy;

  @Field(type = FieldType.Date)
  private Instant time;
}