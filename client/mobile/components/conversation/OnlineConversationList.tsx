import { useEffect, useRef, useState } from "react";
import {
  ActivityIndicator,
  FlatList,
  Keyboard,
  Platform,
  Pressable,
  StyleSheet,
  View,
} from "react-native";
import { colors } from "@/constants";
import { useFocusEffect, useScrollToTop } from "expo-router";
import {
  useGetBlockedConversations,
  useGetInfiniteOnlineConversations,
} from "@/hooks/useConversation";
import SearchInput from "@/components/conversation/SearchInput";
import OnlineConversationItem from "@/components/conversation/OnlineConversationItem";
import { TrueSheet } from "@lodev09/react-native-true-sheet";
import { Ionicons } from "@expo/vector-icons";
import OnlineConversationDetail from "@/components/conversation/OnlineConversationDetail";

export default function OnlineConversationList({
  isActive,
}: {
  isActive: boolean;
}) {
  const [keyword, setKeyword] = useState("");
  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    refetch,
    isFetching,
  } = useGetInfiniteOnlineConversations(keyword);
  const { data: blockedConversations } = useGetBlockedConversations();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const ref = useRef<FlatList | null>(null);
  useScrollToTop(ref);
  const sheet = useRef<TrueSheet>(null);
  const [detailId, setDetailId] = useState("");

  useEffect(() => {
    if (!isActive) {
      sheet.current?.dismiss();
    }
  }, [isActive]);

  useFocusEffect(() => {
    return () => {
      sheet.current?.dismiss();
    };
  });

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await refetch();
    setIsRefreshing(false);
  };

  const handleEndReached = () => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  };

  const handleSearch = (text: string) => {
    setKeyword(text);
  };

  const handleSearchCancel = () => {
    setKeyword("");
    Keyboard.dismiss();
  };

  const handleSheetCloseButtonPress = () => {
    setDetailId("");
    sheet.current?.dismiss();
  };

  return (
    <View style={{ flex: 1 }}>
      <View style={styles.inputContainer}>
        <SearchInput
          placeholder="Search conversation"
          value={keyword}
          onChangeText={(text) => handleSearch(text)}
          submitKeyword={keyword}
          keyword={keyword}
          onCancel={() => handleSearchCancel()}
        />
      </View>
      {isFetching ? (
        <ActivityIndicator style={{ paddingVertical: 50 }} />
      ) : (
        <FlatList
          ref={ref}
          data={data?.pages.flat()}
          renderItem={({ item }) => {
            if (blockedConversations) {
              for (const c of blockedConversations) {
                if (c.id === String(item.id)) {
                  return null;
                }
              }
            }
            return (
              <OnlineConversationItem
                conversation={item}
                onPress={() => {
                  setDetailId(item.id);
                  sheet.current?.present();
                }}
              />
            );
          }}
          keyExtractor={(item, index) => `${item.id}-${index}`}
          contentContainerStyle={styles.contentContainer}
          onEndReached={handleEndReached}
          onEndReachedThreshold={0.5}
          refreshing={isRefreshing}
          onRefresh={handleRefresh}
        />
      )}
      <TrueSheet
        ref={sheet}
        detents={
          Platform.OS === "android" ? [0.123, 0.72] : ["auto", 0.123, 0.72]
        }
        dismissible={false}
        dimmed={false}
        backgroundColor={colors.SAND_100}
      >
        <Pressable
          style={styles.closeButton}
          onPress={() => {
            handleSheetCloseButtonPress();
          }}
        >
          <Ionicons name="close" size={20} color="white" />
        </Pressable>
        <OnlineConversationDetail id={detailId} sheet={sheet} />
      </TrueSheet>
    </View>
  );
}

const styles = StyleSheet.create({
  contentContainer: {
    paddingVertical: 12,
    backgroundColor: colors.SAND_150,
    gap: 12,
  },
  inputContainer: {
    zIndex: 1,
    marginTop: 12,
    paddingHorizontal: 10,
    gap: 8,
    backgroundColor: "transparent",
    flexDirection: "row",
    paddingTop: 0,
    height: 44,
  },
  closeButton: {
    position: "absolute",
    top: 10,
    right: 16,
    zIndex: 1,
    padding: 8,
    backgroundColor: "rgba(0, 0, 0, 0.5)",
    borderRadius: 100,
  },
});
