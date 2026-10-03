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
  useDeleteOnlineConversation,
  useCancelOnlineConversationNotification,
  useDeregisterOnlineConversation,
  useGetOnlineConversationDetail,
  useRegisterOnlineConversation,
  useScheduleOnlineConversationNotification,
} from "@/hooks/useConversation";
import CustomButton from "@/components/CustomButton";
import { colors } from "@/constants";
import Toast from "react-native-toast-message";
import { useActionSheet } from "@expo/react-native-action-sheet";
import { reportOnlineConversation } from "@/api/conversation";
import { router } from "expo-router";
import { requestRecordingPermissionsAsync } from "expo-audio";
import { Ionicons } from "@expo/vector-icons";
import { requestPermissionsAsync } from "expo-notifications";
import { type RefObject } from "react";
import { type TrueSheet } from "@lodev09/react-native-true-sheet";

interface OnlineConversationDetailProps {
  id: string;
  sheet: RefObject<TrueSheet | null>;
}

export default function OnlineConversationDetail({
  id,
  sheet,
}: OnlineConversationDetailProps) {
  const { data } = useGetOnlineConversationDetail({ id });
  const blockConversationMutation = useBlockConversation();
  const { showActionSheetWithOptions } = useActionSheet();
  const registerOnlineConversationMutation = useRegisterOnlineConversation();
  const deregisterOnlineConversationMutation =
    useDeregisterOnlineConversation();
  const scheduleNotificationMutation =
    useScheduleOnlineConversationNotification();
  const cancelNotificationMutation = useCancelOnlineConversationNotification();
  const deleteOnlineConversationMutation = useDeleteOnlineConversation();

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
            deleteOnlineConversationMutation.mutate(
              { id },
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

  const handleReport = () => {
    showActionSheetWithOptions(
      {
        options: [`Report and Delete from feed`, "Cancel"],
        destructiveButtonIndex: 0,
        cancelButtonIndex: 1,
      },
      async (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            blockConversationMutation.mutate({
              id: id,
            });
            try {
              await reportOnlineConversation({ id: id });
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
      },
    );
  };

  const handleEnter = () => {
    router.push({
      pathname: "/online/[id]",
      params: {
        id: id,
      },
    });
  };

  return !data ? (
    <ActivityIndicator style={{ paddingVertical: 50 }} />
  ) : (
    <ScrollView style={{ marginBottom: 100 }}>
      <View style={{ paddingVertical: 30 }}></View>
      <View style={styles.box}>
        <View style={[styles.content]}>
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
          <View style={{ gap: 30 }}>
            {data.canEnter ? (
              <CustomButton
                label="Enter"
                onPress={async () => {
                  await requestRecordingPermissionsAsync();
                  handleEnter();
                }}
              />
            ) : data.isRegistrant ? (
              <View style={styles.buttonRow}>
                <View style={{ flex: 1 }}>
                  <CustomButton
                    label={"Cancel registration"}
                    onPress={() =>
                      deregisterOnlineConversationMutation.mutate({ id })
                    }
                    disabled={deregisterOnlineConversationMutation.isPending}
                  />
                </View>
                {data.isNotificationScheduled ? (
                  <CustomButton
                    label={
                      <Ionicons
                        name="notifications-off"
                        size={20}
                        color={colors.WHITE}
                      />
                    }
                    onPress={() => cancelNotificationMutation.mutate({ id })}
                    disabled={cancelNotificationMutation.isPending}
                    style={styles.iconButton}
                  />
                ) : (
                  <CustomButton
                    label={
                      <Ionicons
                        name="notifications"
                        size={20}
                        color={colors.WHITE}
                      />
                    }
                    onPress={async () => {
                      await requestPermissionsAsync();
                      scheduleNotificationMutation.mutate({ id });
                    }}
                    disabled={scheduleNotificationMutation.isPending}
                    style={styles.iconButton}
                  />
                )}
              </View>
            ) : (
              <CustomButton
                label={"Register"}
                onPress={() =>
                  registerOnlineConversationMutation.mutate({ id: id })
                }
                disabled={registerOnlineConversationMutation.isPending}
              />
            )}
            {data.isModerator && (
              <CustomButton
                label={"Update conversation"}
                onPress={() => {
                  sheet.current?.dismiss();
                  router.push(`/online/update/${id}`);
                }}
              />
            )}
            {data.isModerator && (
              <CustomButton
                label={"Delete conversation"}
                onPress={handleDelete}
                disabled={deleteOnlineConversationMutation.isPending}
              />
            )}
          </View>
        </View>
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
  buttonRow: {
    flexDirection: "row",
    gap: 12,
    alignItems: "center",
    width: "100%",
  },
  iconButton: {
    width: 44,
  },
  reportText: {
    color: colors.GRAY_400,
    fontSize: 12,
  },
  reportPressed: {
    opacity: 0.6,
  },
});
