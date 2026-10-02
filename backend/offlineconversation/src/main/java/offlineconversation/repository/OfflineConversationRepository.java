package offlineconversation.repository;

import offlineconversation.domain.OfflineConversation;
import offlineconversation.dto.UpdateOfflineConversationRequest;
import offlineconversation.projection.OfflineConversationDetailProjection;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

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
          updated_at = UTC_TIMESTAMP()
      WHERE id = :#{#req.id()}
      AND EXISTS (SELECT 1 FROM offline_conversation_moderator
      WHERE conversation_id = :#{#req.id()} AND member_id = :memberId)
      """, nativeQuery = true)
  int updateIfModerator(@Param("req") UpdateOfflineConversationRequest req,
                        @Param("memberId") UUID memberId);
}
