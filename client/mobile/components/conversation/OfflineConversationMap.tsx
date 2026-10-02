import {
  ActivityIndicator,
  FlatList,
  Keyboard,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { AppleMaps, type CameraMoveEvent, GoogleMaps } from "expo-maps";
import { useEffect, useMemo, useRef, useState } from "react";
import { gridDisk, latLngToCell } from "h3-js";
import { Feather, Ionicons } from "@expo/vector-icons";
import {
  useGetBlockedConversations,
  useInfiniteSearchOfflineConversations,
  useMapOfflineConversations,
} from "@/hooks/useConversation";
import {
  getCurrentPositionAsync,
  PermissionStatus,
  requestForegroundPermissionsAsync,
} from "expo-location";
import { useFocusEffect } from "expo-router";
import SearchInput from "@/components/conversation/SearchInput";
import { colors } from "@/constants";
import { TrueSheet } from "@lodev09/react-native-true-sheet";
import OfflineConversationSearchItem from "@/components/conversation/OfflineConversationSearchItem";
import OfflineConversationDetail from "@/components/conversation/OfflineConversationDetail";
import { type OfflineConversationSearchResponse } from "@/types/conversation";

function stripHtml(text: string) {
  if (!text) return "";
  return text.replace(/<\/?[^>]+(>|$)/g, "");
}

export default function OfflineConversationMap({
  isActive,
}: {
  isActive: boolean;
}) {
  const [h3Indexes, setH3Indexes] = useState<string[]>([]);
  const [resolution, setResolution] = useState<number>(7);
  const [position, setPosition] = useState<{
    lat: number;
    lng: number;
    zoom: number;
  } | null>(null);
  const datas = useMapOfflineConversations({
    resolution: resolution,
    h3Indexes: h3Indexes,
  });
  const { data: blockedConversations } = useGetBlockedConversations();
  const [keyword, setKeyword] = useState("");
  const [submitKeyword, setSubmitKeyword] = useState("");
  const [showRetry, setShowRetry] = useState(false);
  const [zoom, setZoom] = useState(0);
  const [submitH3Indexes, setSubmitH3Indexes] = useState<string[]>([]);
  const {
    data: searchData,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isFetching,
  } = useInfiniteSearchOfflineConversations({
    input: submitKeyword,
    resolution: resolution,
    h3Indexes: submitH3Indexes,
  });
  const [detailId, setDetailId] = useState("");
  const sheet = useRef<TrueSheet>(null);

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

  useEffect(() => {
    async function wrapper() {
      const { status } = await requestForegroundPermissionsAsync();
      if (status === PermissionStatus.DENIED) {
        return;
      }
      const location = await getCurrentPositionAsync({});
      setPosition({
        lat: location.coords.latitude,
        lng: location.coords.longitude,
        zoom: Platform.OS === "ios" ? 15 : 17,
      });
    }
    wrapper();
  }, []);

  const markers = useMemo(() => {
    const blockedIds = new Set(
      blockedConversations?.map((block) => block.id) || [],
    );
    return searchData && searchData.pages.flat().length > 0
      ? searchData.pages
          .flat()
          .filter((data) => !blockedIds.has(data.id))
          .map((data) => ({
            id: data.id,
            coordinates: {
              latitude: data.lat,
              longitude: data.lng,
            },
            title: stripHtml(data.writtenBy),
          }))
      : datas
          .filter((data) => !blockedIds.has(data.id))
          .map((data) => ({
            id: data.id,
            coordinates: {
              latitude: data.lat,
              longitude: data.lng,
            },
            title: data.writtenBy,
          }));
  }, [blockedConversations, datas, searchData]);

  const handleEndReached = () => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  };

  const handleCameraMove = (event: CameraMoveEvent) => {
    setShowRetry(true);
    const lat = event.coordinates.latitude;
    const lng = event.coordinates.longitude;
    setZoom(event.zoom);
    if (event.zoom >= (Platform.OS === "ios" ? 12 : 14) && lat && lng) {
      setResolution(7);
      const h3Index = latLngToCell(lat, lng, 7);
      const h3Indexes = gridDisk(h3Index, 1);
      setH3Indexes(h3Indexes);
      return;
    }
    if (lat && lng) {
      setResolution(5);
      const h3Index = latLngToCell(lat, lng, 5);
      const h3Indexes = gridDisk(h3Index, 1);
      setH3Indexes(h3Indexes);
      return;
    }
  };

  const commonSearchFlow = () => {
    setDetailId("");
    setSubmitH3Indexes(h3Indexes);
    setResolution(5);
    if (zoom >= (Platform.OS === "ios" ? 12 : 14)) {
      setResolution(7);
    }
    sheet.current?.present();
  };

  const handleSearch = () => {
    Keyboard.dismiss();
    setShowRetry(false);
    if (!keyword) {
      sheet.current?.dismiss();
      return;
    }
    setSubmitKeyword(keyword);
    commonSearchFlow();
  };

  const handleSearchCancel = () => {
    setSubmitKeyword("");
    setKeyword("");
    setSubmitH3Indexes([]);
    sheet.current?.dismiss();
  };

  const handleRetrySearch = () => {
    setKeyword(submitKeyword);
    Keyboard.dismiss();
    setShowRetry(false);
    commonSearchFlow();
  };

  const handleSheetCloseButtonPress = () => {
    if (detailId && submitKeyword) {
      setDetailId("");
      return;
    }
    if (detailId) {
      setDetailId("");
      sheet.current?.dismiss();
    }
    if (submitKeyword) {
      handleSearchCancel();
    }
  };

  const handleSearchItemPress = (item: OfflineConversationSearchResponse) => {
    setDetailId(item.id);
    setPosition(null);
    setTimeout(() => {
      setPosition({ lat: item.lat, lng: item.lng, zoom: 15 });
    }, 0);
  };

  return (
    <View style={{ flex: 1 }}>
      <View style={styles.inputContainer}>
        <SearchInput
          placeholder="Search conversation"
          value={keyword}
          onChangeText={(text) => setKeyword(text)}
          onSubmit={() => {
            handleSearch();
          }}
          submitKeyword={submitKeyword}
          keyword={keyword}
          onCancel={() => {
            handleSearchCancel();
          }}
        />
      </View>
      {showRetry && submitKeyword && (
        <View style={styles.retrySearchContainer}>
          <Pressable
            style={styles.retrySearchButton}
            onPress={() => {
              handleRetrySearch();
            }}
          >
            <Feather name="rotate-cw" size={14} color="black" />
            <Text style={styles.retrySearchText}>Search this area</Text>
          </Pressable>
        </View>
      )}
      {Platform.OS === "ios" ? (
        <AppleMaps.View
          style={StyleSheet.absoluteFill}
          cameraPosition={{
            coordinates: {
              latitude: position?.lat,
              longitude: position?.lng,
            },
            zoom: position?.zoom,
          }}
          onCameraMove={(event) => {
            handleCameraMove(event);
          }}
          markers={markers}
          onMapClick={() => {
            Keyboard.dismiss();
            setDetailId("");
          }}
          onMarkerClick={(event) => {
            sheet.current?.present();
            setDetailId(String(event.id));
          }}
        />
      ) : (
        <GoogleMaps.View
          style={StyleSheet.absoluteFill}
          cameraPosition={{
            coordinates: {
              latitude: position?.lat,
              longitude: position?.lng,
            },
            zoom: position?.zoom,
          }}
          onMapClick={() => {
            Keyboard.dismiss();
            setDetailId("");
          }}
          onMarkerClick={(event) => {
            sheet.current?.present();
            setDetailId(String(event.id));
          }}
          onCameraMove={(event) => {
            handleCameraMove(event);
          }}
          uiSettings={{
            compassEnabled: true,
            myLocationButtonEnabled: true,
            zoomControlsEnabled: false,
            rotationGesturesEnabled: true,
          }}
          markers={markers}
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
        {detailId ? (
          <View style={styles.container}>
            <OfflineConversationDetail id={detailId} sheet={sheet}/>
          </View>
        ) : isFetching ? (
          <ActivityIndicator style={{ paddingVertical: 50 }} />
        ) : (
          <>
            <View style={{ paddingVertical: 30 }}></View>
            <FlatList
              data={searchData?.pages.flat() || []}
              ListEmptyComponent={
                <View style={styles.emptyContainer}>
                  <Text style={styles.emptyText}>No result</Text>
                </View>
              }
              renderItem={({ item }) => {
                if (blockedConversations) {
                  for (const b of blockedConversations) {
                    if (b.id === String(item.id)) {
                      return null;
                    }
                  }
                }
                return (
                  <OfflineConversationSearchItem
                    conversation={item}
                    onPress={() => {
                      handleSearchItemPress(item);
                    }}
                  />
                );
              }}
              keyExtractor={(item) => String(item.id)}
              contentContainerStyle={styles.contentContainer}
              onEndReached={handleEndReached}
              onEndReachedThreshold={0.5}
            />
          </>
        )}
      </TrueSheet>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingVertical: 57,
  },
  inputContainer: {
    zIndex: 1,
    marginTop: Platform.OS === "android" ? 10 : 17,
    marginRight: Platform.OS === "android" ? 18 : 75,
    paddingLeft: Platform.OS === "android" ? 55 : 10,
    gap: 8,
    backgroundColor: "transparent",
    flexDirection: "row",
    paddingTop: 0,
    height: 44,
  },
  retrySearchContainer: {
    marginTop: 20,
    alignItems: "center",
    zIndex: 10,
    marginHorizontal: 100,
  },
  retrySearchButton: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: colors.WHITE,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 24,
  },
  retrySearchText: {
    marginLeft: 8,
    fontSize: 14,
    fontWeight: "600",
    color: colors.BLACK,
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
  contentContainer: {
    paddingVertical: 12,
    backgroundColor: colors.SAND_150,
    gap: 12,
  },
  emptyContainer: {
    paddingVertical: 40,
    alignItems: "center",
    justifyContent: "center",
  },
  emptyText: {
    fontSize: 16,
    color: colors.BLACK,
    fontWeight: "500",
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
