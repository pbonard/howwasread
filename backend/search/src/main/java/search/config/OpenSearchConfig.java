package search.config;

import org.opensearch.data.client.osc.OpenSearchConfiguration;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.elasticsearch.client.ClientConfiguration;


@Configuration
public class OpenSearchConfig extends OpenSearchConfiguration {

  @Value("${spring.elasticsearch.uris}")
  private String uri;

  @Value("${spring.elasticsearch.username}")
  private String username;

  @Value("${spring.elasticsearch.password}")
  private String password;

  // the in cluster opensearch serves plain http, a hosted one like aiven needs https
  @Value("${opensearch.ssl}")
  private boolean ssl;

  @Override
  @Bean
  public ClientConfiguration clientConfiguration() {
    var builder = ClientConfiguration.builder().connectedTo(uri);
    return (ssl ? builder.usingSsl() : builder)
        .withBasicAuth(username, password)
        .build();
  }
}