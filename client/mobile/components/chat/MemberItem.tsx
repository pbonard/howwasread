import { Pressable, StyleSheet, Text, View } from "react-native";
import { useGetProfile } from "@/hooks/useProfile";
import { getKVStore } from "@/db/storage";
import { colors } from "@/constants";
import { router } from "expo-router";
import { useActionSheet } from "@expo/react-native-action-sheet";
import { useReportUser } from "@/hooks/useChat";
import Toast from "react-native-toast-message";

export default function MemberItem({ id }: { id: string }) {
  const { data } = useGetProfile(id);
  const { showActionSheetWithOptions } = useActionSheet();
  const reportUserMutation = useReportUser();

  const displayName = data?.name || getKVStore(id);

  function handleLongPress() {
    id !== getKVStore("myId") &&
      showActionSheetWithOptions(
        {
          options: ["Report", "Cancel"],
          destructiveButtonIndex: 0,
          cancelButtonIndex: 2,
        },
        (selectedIndex?: number) => {
          console.log(selectedIndex);
          switch (selectedIndex) {
            case 0:
              reportUserMutation.mutate(
                {
                  id: id,
                },
                {
                  onSuccess: () => {
                    Toast.show({
                      type: "info",
                      text1: "Success report",
                      text2:
                        "We will review this user, sorry for inconvenience.",
                    });
                  },
                },
              );
              break;
          }
        },
      );
  }

  return (
    <Pressable
      style={({ pressed }) => [styles.container, pressed && styles.pressed]}
      onPress={() =>
        router.push(
          { pathname: "/chat/[id]", params: { id: id } },
          { dangerouslySingular: (_, params) => String(params.id) },
        )
      }
      onLongPress={() => handleLongPress()}
    >
      <View style={styles.avatar}>
        <Text style={styles.avatarText}>
          {displayName.charAt(0).toUpperCase()}
        </Text>
      </View>
      <View style={styles.nameRow}>
        {getKVStore("myId") === id && <Text style={styles.youText}>you</Text>}
        <Text style={styles.nameText}>{displayName}</Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    alignItems: "center",
    paddingVertical: 12,
    paddingHorizontal: 16,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#E5E5E5",
  },
  pressed: {
    backgroundColor: "#F3F4F6",
  },
  avatar: {
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: colors.ORANGE_150,
    justifyContent: "center",
    alignItems: "center",
    marginRight: 16,
  },
  avatarText: {
    fontSize: 18,
    fontWeight: "600",
    color: colors.GRAY_700,
  },
  nameRow: {
    flexDirection: "row",
    alignItems: "center",
  },
  youText: {
    fontSize: 13,
    fontWeight: "400",
    color: "#9CA3AF",
    marginRight: 6,
  },
  nameText: {
    fontSize: 16,
    fontWeight: "500",
    color: "#1F2937",
  },
});
