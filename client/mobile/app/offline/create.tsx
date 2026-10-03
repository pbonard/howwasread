import { useCreateOfflineConversation } from "@/hooks/useConversation";
import Toast from "react-native-toast-message";
import { router } from "expo-router";
import OfflineConversationForm, {
  emptyOfflineConversationForm,
} from "@/components/conversation/OfflineConversationForm";

export default function OfflineConversationScreen() {
  const createOfflineConversationMutation = useCreateOfflineConversation();
  return (
    <OfflineConversationForm
      defaultValues={emptyOfflineConversationForm()}
      submitLabel="Create"
      isPending={createOfflineConversationMutation.isPending}
      onSubmit={(request) =>
        createOfflineConversationMutation.mutate(request, {
          onSuccess: () => {
            router.replace("/conversations");
          },
          onError: (error) => {
            console.log(error);
            Toast.show({
              type: "error",
              text1: "Invalid Google Maps URL",
            });
          },
        })
      }
    />
  );
}
