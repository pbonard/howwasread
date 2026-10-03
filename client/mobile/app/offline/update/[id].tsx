import { ActivityIndicator, StyleSheet, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import Toast from "react-native-toast-message";
import {
  useGetOfflineConversationDetail,
  useUpdateOfflineConversation,
} from "@/hooks/useConversation";
import OfflineConversationForm, {
  toOfflineConversationForm,
} from "@/components/conversation/OfflineConversationForm";
import { colors } from "@/constants";

export default function OfflineConversationUpdateScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data } = useGetOfflineConversationDetail(id);
  const updateOfflineConversationMutation = useUpdateOfflineConversation();

  // the form takes its default values once, so it is rendered after the detail is loaded
  if (!data) {
    return (
      <View style={styles.loading}>
        <ActivityIndicator />
      </View>
    );
  }
  return (
    <OfflineConversationForm
      defaultValues={toOfflineConversationForm(data)}
      submitLabel="Update"
      isPending={updateOfflineConversationMutation.isPending}
      onSubmit={(request) =>
        updateOfflineConversationMutation.mutate(
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
