import { Platform, StatusBar, StyleSheet, Text, View } from "react-native";
import { useState } from "react";
import { colors } from "@/constants";
import { Feather, SimpleLineIcons } from "@expo/vector-icons";
import { router } from "expo-router";

interface OnlineConversationRoomHeaderProps {
  novel?: string;
  shortStory?: string;
  poem?: string;
  play?: string;
  film?: string;
  writtenBy?: string;
  description?: string;
  time: string;
  length: string;
}

export default function OnlineConversationRoomHeader({
  novel,
  shortStory,
  poem,
  play,
  film,
  writtenBy,
  description,
  time,
  length,
}: OnlineConversationRoomHeaderProps) {
  const [open, setOpen] = useState(false);

  return (
    <>
      {open ? (
        <View style={styles.content}>
          <View>
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
                .format(new Date(time))
                .replace(/\sat\s/, " ")}
              {` For ${length.replace("0s", "")}`}
            </Text>
            {!!novel && <Text style={styles.detail}>Novel: {novel}</Text>}
            {!!shortStory && (
              <Text style={styles.detail}>Short story: {shortStory}</Text>
            )}
            {!!poem && <Text style={styles.detail}>Poem: {poem}</Text>}
            {!!play && <Text style={styles.detail}>Play: {play}</Text>}
            {!!film && <Text style={styles.detail}>Film: {film}</Text>}
            {!!writtenBy && <Text style={styles.detail}>By: {writtenBy}</Text>}
            {!!description ? (
              <View>
                <Text style={styles.descriptionHeader}>Description</Text>{" "}
                <Text style={styles.detail}>{description}</Text>
              </View>
            ) : (
              <Text style={styles.descriptionHeader}>No description</Text>
            )}
          </View>
          <SimpleLineIcons
            name="arrow-up"
            size={24}
            color="black"
            onPress={() => setOpen(false)}
          />
        </View>
      ) : (
        <View>
          <View style={styles.headerRow}>
            <View style={styles.exitButton}>
              <Feather
                name="arrow-left"
                size={28}
                color={colors.BLACK}
                onPress={() => router.back()}
              />
            </View>
            <View style={styles.headerCenter}>
              <Text style={styles.headerText}>Exchange ideas</Text>
              <SimpleLineIcons
                name="arrow-down"
                size={25}
                color="black"
                onPress={() => setOpen(true)}
              />
            </View>
          </View>
        </View>
      )}
    </>
  );
}

const styles = StyleSheet.create({
  content: {
    justifyContent: "center",
    alignItems: "center",
    padding: 16,
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
  headerRow: {
    position: "relative",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 8,
    paddingHorizontal: 16,
    gap: 8,
    backgroundColor: colors.SAND_110,
    flexDirection: "row",
    paddingTop: Platform.OS === "android" ? StatusBar.currentHeight : 0,
    height: Platform.OS === "ios" ? 44 : 65,
  },
  exitButton: {
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    zIndex: 1,
  },
  headerCenter: {
    position: "absolute",
    left: 0,
    right: 0,
    top: 0,
    bottom: 0,
    alignItems: "center",
    justifyContent: "center",
  },
  headerText: {
    fontSize: 23,
    color: colors.BLACK,
    fontWeight: 300,
  },
});
