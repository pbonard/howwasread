import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
} from "@tanstack/react-query";
import {
  banParticipant,
  blockConversation,
  cancelOnlineConversationNotification,
  createOfflineConversation,
  createOnlineConversation,
  deleteOfflineConversation,
  deleteOnlineConversation,
  deregisterOnlineConversation,
  getBlockedConversations,
  getOfflineConversationDetail,
  getOnlineConversationDetail,
  getOnlineConversations,
  getTurn,
  joinOfflineConversation,
  mapOfflineConversations,
  quitOfflineConversation,
  registerOnlineConversation,
  scheduleOnlineConversationNotification,
  searchOfflineConversations,
  searchOnlineConversations,
  updateOfflineConversation,
  updateOnlineConversation,
} from "@/api/conversation";
import { queryKey } from "@/constants";
import { AxiosError } from "axios";
import Toast from "react-native-toast-message";
import queryClient from "@/api/queryClient";
import { OfflineConversationMapResponse } from "@/types/conversation";

export function useGetInfiniteOnlineConversations(input: string) {
  return useInfiniteQuery({
    queryFn: ({ pageParam }) => {
      if (input) {
        return searchOnlineConversations({
          input,
          page: pageParam,
        });
      }
      return getOnlineConversations(pageParam);
    },

    queryKey: [
      queryKey.CONVERSATION,
      queryKey.SEARCH_ONLINE_CONVERSATIONS,
      input,
    ],
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => {
      const lastPost = lastPage[lastPage.length - 1];
      return lastPost ? allPages.length + 1 : undefined;
    },
  });
}

export function useCreateOnlineConversation() {
  return useMutation({
    mutationFn: createOnlineConversation,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.GET_ONLINE_CONVERSATIONS],
      });
    },
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useUpdateOnlineConversation() {
  return useMutation({
    mutationFn: updateOnlineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.SEARCH_ONLINE_CONVERSATIONS],
      });
    },
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useDeleteOnlineConversation() {
  return useMutation({
    mutationFn: deleteOnlineConversation,
    onSuccess: async (data, variables) => {
      // removed, not invalidated, a refetch of a deleted conversation fails
      queryClient.removeQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.SEARCH_ONLINE_CONVERSATIONS],
      });
    },
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useBanParticipant() {
  return useMutation({
    mutationFn: banParticipant,
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useMapOfflineConversations({
  resolution,
  h3Indexes,
}: {
  resolution: number;
  h3Indexes: string[];
}): OfflineConversationMapResponse[] {
  const queries = useQueries({
    queries: h3Indexes.map((h3Index) => {
      return {
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.MAP_OFFLINE_CONVERSATIONS,
          h3Index,
        ],
        queryFn: () => mapOfflineConversations({ resolution, h3Index }),
      };
    }),
  });

  return queries.flatMap((query) => {
    if (query.data) {
      return query.data;
    }
    return [];
  });
}

export function useInfiniteSearchOfflineConversations({
  input,
  resolution,
  h3Indexes,
}: {
  input: string;
  resolution: number;
  h3Indexes: string[];
}) {
  return useInfiniteQuery({
    queryFn: ({ pageParam }) =>
      searchOfflineConversations({
        input,
        resolution,
        h3Indexes,
        page: pageParam,
      }),
    queryKey: [
      queryKey.CONVERSATION,
      queryKey.SEARCH_OFFLINE_CONVERSATIONS,
      input,
      h3Indexes,
    ],
    enabled: h3Indexes.length > 0 && !!input,
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => {
      const lastPost = lastPage[lastPage.length - 1];
      return lastPost ? allPages.length + 1 : undefined;
    },
  });
}

export function useCreateOfflineConversation() {
  return useMutation({
    mutationFn: createOfflineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.MAP_OFFLINE_CONVERSATIONS,
          variables?.h3Res5,
        ],
      });
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.MAP_OFFLINE_CONVERSATIONS,
          variables?.h3Res7,
        ],
      });
    },
  });
}

export function useUpdateOfflineConversation() {
  return useMutation({
    mutationFn: updateOfflineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.MAP_OFFLINE_CONVERSATIONS],
      });
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.SEARCH_OFFLINE_CONVERSATIONS,
        ],
      });
    },
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useDeleteOfflineConversation() {
  return useMutation({
    mutationFn: deleteOfflineConversation,
    onSuccess: async (data, variables) => {
      // removed, not invalidated, a refetch of a deleted conversation fails
      queryClient.removeQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
          variables.conversationId,
        ],
      });
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.MAP_OFFLINE_CONVERSATIONS],
      });
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.SEARCH_OFFLINE_CONVERSATIONS,
        ],
      });
    },
    onError: (error: AxiosError) => {
      console.log(error?.response?.data);
      Toast.show({
        type: "error",
        text1: String(error?.response?.data),
      });
    },
  });
}

export function useGetOfflineConversationDetail(id: string) {
  const { data } = useQuery({
    queryFn: () => getOfflineConversationDetail(id),
    queryKey: [
      queryKey.CONVERSATION,
      queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
      id,
    ],
    enabled: !!id,
  });
  return { data };
}

export function useJoinOfflineConversation() {
  return useMutation({
    mutationFn: joinOfflineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
          variables.conversationId,
        ],
      });
    },
  });
}

export function useQuitOfflineConversation() {
  return useMutation({
    mutationFn: quitOfflineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
          variables.conversationId,
        ],
      });
    },
  });
}

export function useGetOnlineConversationDetail({
  id,
  isPersonal,
}: {
  id: string;
  isPersonal?: boolean;
}) {
  return useQuery({
    queryFn: () => getOnlineConversationDetail(id),
    queryKey: [
      queryKey.CONVERSATION,
      queryKey.GET_ONLINE_CONVERSATION_DETAIL,
      id,
    ],
    enabled: !!id && !isPersonal,
  });
}

export function useBlockConversation() {
  return useMutation({
    mutationFn: blockConversation,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: [queryKey.CONVERSATION, queryKey.BLOCKED_CONVERSATIONS],
      });
    },
  });
}

export function useGetBlockedConversations() {
  return useQuery({
    queryFn: getBlockedConversations,
    queryKey: [queryKey.CONVERSATION, queryKey.BLOCKED_CONVERSATIONS],
  });
}

export function useRegisterOnlineConversation() {
  return useMutation({
    mutationFn: registerOnlineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
    },
    onError: (error: AxiosError) => {
      Toast.show({
        type: "error",
        text1: String(error.response?.data),
      });
    },
  });
}

export function useDeregisterOnlineConversation() {
  return useMutation({
    mutationFn: deregisterOnlineConversation,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
    },
  });
}

export function useGetTurn() {
  return useQuery({
    queryFn: getTurn,
    queryKey: [queryKey.CONVERSATION, queryKey.GET_TURN],
  });
}

export function useScheduleOnlineConversationNotification() {
  return useMutation({
    mutationFn: scheduleOnlineConversationNotification,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
      Toast.show({
        type: "success",
        text1: "We will send push 15 minutes before convo start",
      });
    },
  });
}

export function useCancelOnlineConversationNotification() {
  return useMutation({
    mutationFn: cancelOnlineConversationNotification,
    onSuccess: async (data, variables) => {
      await queryClient.invalidateQueries({
        queryKey: [
          queryKey.CONVERSATION,
          queryKey.GET_ONLINE_CONVERSATION_DETAIL,
          variables.id,
        ],
      });
    },
  });
}
