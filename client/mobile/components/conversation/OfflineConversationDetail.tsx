import {
  ActivityIndicator,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import {
  useBlockConversation,
  useDeleteOfflineConversation,
  useGetOfflineConversationDetail,
  useJoinOfflineConversation,
  useQuitOfflineConversation,
} from "@/hooks/useConversation";
import CustomButton from "@/components/CustomButton";
import { colors } from "@/constants";
import { openBrowserAsync } from "expo-web-browser";
import Toast from "react-native-toast-message";
import { router } from "expo-router";
import { useActionSheet } from "@expo/react-native-action-sheet";
import { reportOfflineConversation } from "@/api/conversation";
import { type RefObject } from "react";
import { TrueSheet } from "@lodev09/react-native-true-sheet";

interface OfflineConversationDetailProps {
  id: string;
  sheet: RefObject<TrueSheet | null>;
}

export default function OfflineConversationDetail({
  id,
  sheet,
}: OfflineConversationDetailProps) {
  const { data } = useGetOfflineConversationDetail(id);
  const joinOfflineConversationMutation = useJoinOfflineConversation();
  const quitOfflineConversationMutation = useQuitOfflineConversation();
  const blockConversationMutation = useBlockConversation();
  const deleteOfflineConversationMutation = useDeleteOfflineConversation();

  const { showActionSheetWithOptions } = useActionSheet();

  const handleReport = () => {
    showActionSheetWithOptions(
      {
        options: [`Report and Delete from map`, "Cancel"],
        destructiveButtonIndex: 0,
        cancelButtonIndex: 4,
      },
      async (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            blockConversationMutation.mutate({
              id: String(id),
            });
            if (data) {
              try {
                await reportOfflineConversation({ conversationId: String(id) });
              } catch (e) {
                console.log(e);
              }
              Toast.show({
                type: "info",
                text1: "Success report",
                text2:
                  "We will review this conversation, sorry for inconvenience.",
              });
              sheet.current?.dismiss();
            }
        }
      },
    );
  };

  const handleDelete = () => {
    showActionSheetWithOptions(
      {
        options: ["Delete conversation", "Cancel"],
        destructiveButtonIndex: 0,
        cancelButtonIndex: 1,
      },
      (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            deleteOfflineConversationMutation.mutate(
              { conversationId: id },
              {
                onSuccess: () => {
                  Toast.show({
                    type: "success",
                    text1: "Conversation deleted",
                  });
                  sheet.current?.dismiss();
                },
              },
            );
        }
      },
    );
  };

  const handleButtonPress = () => {
    if (data?.isParticipant) {
      quitOfflineConversationMutation.mutate(
        { conversationId: id },
        { onSuccess: () => router.push("/conversations") },
      );
      return;
    }
    joinOfflineConversationMutation.mutate(
      { conversationId: String(id) },
      {
        onSuccess: () => {
          Toast.show({
            type: "success",
            text1: "We invite you to the group chat room!",
          });
        },
      },
    );
  };

  return !data ? (
    <ActivityIndicator style={{ paddingVertical: 50 }} />
  ) : (
    <ScrollView style={{ marginBottom: 100 }}>
      <View style={styles.box}>
        <View style={[styles.content, data.isModerator && styles.moderator]}>
          <Text style={styles.when}>
            {new Intl.DateTimeFormat("en-US", {
              weekday: "short",
              year: "numeric",
              month: "short",
              day: "numeric",
              hour: "2-digit",
              minute: "2-digit",
              hourCycle: "h12",
            })
              .format(new Date(data.time))
              .replace(/\sat\s/, " ")}
            {`\nFor ${data.lengthMinutes}m`}
          </Text>
          {data.novel && <Text style={styles.detail}>Novel: {data.novel}</Text>}
          {data.shortStory && (
            <Text style={styles.detail}>Short story: {data.shortStory}</Text>
          )}
          {data.poem && <Text style={styles.detail}>Poem: {data.poem}</Text>}
          {data.play && <Text style={styles.detail}>Play: {data.play}</Text>}
          {data.film && <Text style={styles.detail}>Film: {data.film}</Text>}
          <Text style={styles.detail}>Written by: {data.writtenBy}</Text>
          {data.description ? (
            <View>
              <Text style={styles.descriptionHeader}>Description</Text>{" "}
              <Text style={styles.detail}>{data.description}</Text>
            </View>
          ) : (
            <Text style={styles.descriptionHeader}>No description</Text>
          )}
          {
            <Text style={styles.detail}>
              number of participants: {data.numberOfParticipants}
            </Text>
          }
          {data.location && (
            <Text style={styles.detail}>Location: {data.location}</Text>
          )}
          <Text
            style={[styles.detail, { color: colors.BLUE_500 }]}
            onPress={() => openBrowserAsync(data.mapsLink)}
          >
            Maps link
          </Text>
        </View>
        <CustomButton
          label={
            data.isParticipant
              ? "Quit offline conversation"
              : "Join offline conversation"
          }
          onPress={handleButtonPress}
          style={{ paddingHorizontal: 20 }}
          disabled={
            joinOfflineConversationMutation.isPending ||
            quitOfflineConversationMutation.isPending
          }
        />
        {data.isModerator && (
          <CustomButton
            label={"Update conversation"}
            onPress={() => {
              sheet.current?.dismiss();
              router.push(`/offline/update/${id}`);
            }}
            style={{ paddingHorizontal: 20 }}
          />
        )}
        {data.isModerator && (
          <CustomButton
            label={"Delete conversation"}
            onPress={handleDelete}
            style={{ paddingHorizontal: 20 }}
            disabled={deleteOfflineConversationMutation.isPending}
          />
        )}
      </View>
      <View style={styles.footer}>
        <Pressable
          onPress={async () => handleReport()}
          style={({ pressed }) => [pressed && styles.reportPressed]}
        >
          <Text style={styles.reportText}>Report conversation</Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}
const styles = StyleSheet.create({
  container: { backgroundColor: colors.SAND_110 },
  box: {
    padding: 16,
    marginHorizontal: 16,
    backgroundColor: "#ffffff",
    borderRadius: 8,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 2,
  },
  footer: {
    marginVertical: 50,
    alignItems: "center",
  },
  content: {
    padding: 16,
    gap: 17,
  },
  when: {
    fontSize: 19,
    color: colors.BLACK,
    fontWeight: 500,
    marginVertical: 6,
  },
  detail: {
    fontSize: 17,
    fontWeight: 300,
  },
  descriptionHeader: {
    marginTop: 6,
    fontSize: 18,
    fontWeight: 400,
  },
  moderator: {
    backgroundColor: colors.SAND_110,
  },
  reportText: {
    color: colors.GRAY_400,
    fontSize: 12,
  },
  reportPressed: {
    opacity: 0.6,
  },
});
