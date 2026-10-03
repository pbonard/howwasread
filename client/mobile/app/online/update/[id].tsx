import { ActivityIndicator, StyleSheet, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import Toast from "react-native-toast-message";
import {
  useGetOnlineConversationDetail,
  useUpdateOnlineConversation,
} from "@/hooks/useConversation";
import OnlineConversationForm, {
  toOnlineConversationForm,
} from "@/components/conversation/OnlineConversationForm";
import { colors } from "@/constants";

export default function OnlineConversationUpdateScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data } = useGetOnlineConversationDetail({ id });
  const updateOnlineConversationMutation = useUpdateOnlineConversation();

  // the form takes its default values once, so it is rendered after the detail is loaded
  if (!data) {
    return (
      <View style={styles.loading}>
        <ActivityIndicator />
      </View>
    );
  }
  return (
    <OnlineConversationForm
      defaultValues={toOnlineConversationForm(data)}
      submitLabel="Update"
      isPending={updateOnlineConversationMutation.isPending}
      onSubmit={(request) =>
        updateOnlineConversationMutation.mutate(
          { ...request, id },
          {
            onSuccess: () => {
              Toast.show({ type: "success", text1: "Conversation updated" });
              router.back();
            },
          },
        )
      }
    />
  );
}

const styles = StyleSheet.create({
  loading: {
    flex: 1,
    justifyContent: "center",
    backgroundColor: colors.SAND_110,
  },
});
