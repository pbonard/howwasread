package offlineconversation.repository;

import offlineconversation.domain.OfflineConversation;
import offlineconversation.dto.UpdateOfflineConversationRequest;
import offlineconversation.dto.Verdict;
import offlineconversation.projection.OfflineConversationDetailProjection;
import offlineconversation.projection.ReportTargetProjection;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

@Repository
public interface OfflineConversationRepository extends JpaRepository<OfflineConversation, UUID> {

  @Query(value = """
      SELECT c.novel as novel, c.poem as poem, c.short_story as shortStory,
      c.play as play, c.film as film, c.written_by as writtenBy, c.description as description,
      c.time as time, c.length_minutes as lengthMinutes, c.maps_link as mapsLink,
      c.location as location, c.updated_at as updatedAt,
      EXISTS(SELECT 1 FROM offline_conversation_moderator m
      WHERE m.conversation_id = c.id
      AND m.member_id = :memberId) as isModerator,
      EXISTS(SELECT 1 FROM offline_conversation_participant p
      WHERE p.conversation_id = c.id
      AND p.member_id = :memberId) as isParticipant,
      (SELECT COUNT(*) FROM offline_conversation_participant p2
      WHERE p2.conversation_id = c.id) as numberOfParticipants
      FROM offline_conversation c
      WHERE c.id = :conversationId
      """, nativeQuery = true)
  Optional<OfflineConversationDetailProjection> findDetail(
      @Param("conversationId") UUID conversationId,
      @Param("memberId") UUID memberId
  );

  @Modifying
  @Query(value = """
      DELETE FROM offline_conversation
      WHERE id=:conversationId
      AND EXISTS (SELECT 1 FROM offline_conversation_moderator
      WHERE conversation_id=:conversationId AND member_id=:memberId)
      """, nativeQuery = true)
  int deleteIfModerator(
      @Param("conversationId") UUID conversationId,
      @Param("memberId") UUID memberId
  );

  @Modifying(clearAutomatically = true)
  @Query(value = """
      UPDATE offline_conversation
      SET novel = :#{#req.novel()}, short_story = :#{#req.shortStory()}, poem = :#{#req.poem()},
          play = :#{#req.play()}, film = :#{#req.film()},
          written_by = :#{#req.writtenBy()}, description = :#{#req.description()},
          time = :#{#req.time()}, length_minutes = :#{#req.lengthMinutes()},
          updated_at = UTC_TIMESTAMP(6), is_evaluated = FALSE
      WHERE id = :#{#req.id()}
      AND EXISTS (SELECT 1 FROM offline_conversation_moderator
      WHERE conversation_id = :#{#req.id()} AND member_id = :memberId)
      """, nativeQuery = true)
  int updateIfModerator(@Param("req") UpdateOfflineConversationRequest req,
                        @Param("memberId") UUID memberId);

  @Query(value = """
      SELECT novel, short_story as shortStory, poem, play, film, written_by as writtenBy,
      description, location, updated_at as updatedAt, CAST(is_evaluated AS SIGNED) as isEvaluated
      FROM offline_conversation
      WHERE id = :conversationId
      """, nativeQuery = true)
  Optional<ReportTargetProjection> findReportTarget(@Param("conversationId") UUID conversationId);

  // <=> matches a NULL updated_at too, the row is marked only when nobody updated it during the evaluation
  @Modifying
  @Query(value = """
      UPDATE offline_conversation SET is_evaluated = TRUE
      WHERE id = :conversationId AND updated_at <=> :updatedAt
      """, nativeQuery = true)
  int markEvaluatedIfUnchanged(@Param("conversationId") UUID conversationId,
                               @Param("updatedAt") Instant updatedAt);

  @Modifying
  @Query(value = """
      INSERT INTO offline_conversation_verdict
      (conversation_id, content_updated_at, contents, model, violation, category, reason)
      VALUES (:conversationId, :contentUpdatedAt, :contents, :model, :#{#v.violation()}, :#{#v.category()}, :#{#v.reason()})
      """, nativeQuery = true)
  void insertVerdict(@Param("conversationId") UUID conversationId,
                     @Param("contentUpdatedAt") Instant contentUpdatedAt,
                     @Param("contents") String contents,
                     @Param("model") String model,
                     @Param("v") Verdict v);

  @Modifying
  @Query(value = "DELETE FROM offline_conversation WHERE id = :conversationId", nativeQuery = true)
  void deleteConversation(@Param("conversationId") UUID conversationId);

  @Modifying
  @Query(value = "DELETE FROM offline_conversation_participant WHERE conversation_id = :conversationId", nativeQuery = true)
  void deleteParticipants(@Param("conversationId") UUID conversationId);

  @Modifying
  @Query(value = "DELETE FROM offline_conversation_moderator WHERE conversation_id = :conversationId", nativeQuery = true)
  void deleteModerators(@Param("conversationId") UUID conversationId);
}
