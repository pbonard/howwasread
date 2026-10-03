import { useCreateOnlineConversation } from "@/hooks/useConversation";
import { router } from "expo-router";
import OnlineConversationForm, {
  emptyOnlineConversationForm,
} from "@/components/conversation/OnlineConversationForm";

export default function OnlineConversationCreateScreen() {
  const createOnlineConversationMutation = useCreateOnlineConversation();
  return (
    <OnlineConversationForm
      defaultValues={emptyOnlineConversationForm()}
      submitLabel="Create"
      isPending={createOnlineConversationMutation.isPending}
      onSubmit={(request) =>
        createOnlineConversationMutation.mutate(request, {
          onSuccess: () => {
            router.replace("/conversations");
          },
        })
      }
    />
  );
}
