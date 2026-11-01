import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router } from "expo-router";
import { useActionSheet } from "@expo/react-native-action-sheet";
import { useDeleteAccount, useLogout } from "@/hooks/useAuth";
import { colors } from "@/constants";
import { deleteSecure, removeKVStore } from "@/db/storage";
import { deleteAllMessages } from "@/db/message";
import { useSQLiteContext } from "expo-sqlite";

export default function AccountScreen() {
  const { showActionSheetWithOptions } = useActionSheet();
  const deleteAccountMutation = useDeleteAccount();
  const logoutMutation = useLogout();
  const db = useSQLiteContext();
  const handleDeleteAccount = () => {
    showActionSheetWithOptions(
      {
        title: "This is irreversible",
        options: [`Delete your account`, "Cancel"],
        cancelButtonIndex: 1,
        destructiveButtonIndex: 0
      },
      async (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            deleteAccountMutation.mutate(undefined);
            await deleteAllMessages(db);
            await deleteSecure("accessToken");
            removeKVStore("myId");
            removeKVStore("myName");
            removeKVStore("recentMessageId");
            router.replace("/");
            break;
          case 1:
            break;
        }
      }
    );
  };
  const handleLogout = () => {
    showActionSheetWithOptions(
      {
        title: "This will delete your local messages.",
        options: [`Logout`, "Cancel"],
        cancelButtonIndex: 2,
        destructiveButtonIndex: 0
      },
      async (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            logoutMutation.mutate(undefined);
            await deleteAllMessages(db);
            await deleteSecure("accessToken");
            removeKVStore("myId");
            removeKVStore("myName");
            removeKVStore("recentMessageId");
            router.replace("/");
            break;
          case 1:
            break;
        }
      }
    );
  };
  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.menuContainer}>
        <Pressable
          onPress={() =>
            router.push("/profile/name", { dangerouslySingular: true })
          }
          style={({ pressed }) => [
            styles.menuItem,
            pressed && styles.menuItemPressed
          ]}
        >
          <Text style={styles.menuText}>Change your name</Text>
        </Pressable>
        <Pressable
          onPress={handleLogout}
          style={({ pressed }) => [
            styles.menuItem,
            pressed && styles.menuItemPressed
          ]}
        >
          <Text style={styles.menuText}>Sign out</Text>
        </Pressable>
      </View>
      <View style={styles.footer}>
        <Pressable
          onPress={handleDeleteAccount}
          style={({ pressed }) => [pressed && styles.deletePressed]}
        >
          <Text style={styles.deleteText}>Delete your account</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: colors.SAND_110
  },
  menuContainer: {
    flex: 1,
    gap: 10
  },
  menuItem: {
    padding: 20,
    borderWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.BLACK
  },
  menuItemPressed: {
    opacity: 0.6
  },
  menuText: {
    fontSize: 18,
    color: colors.BLACK
  },
  footer: {
    position: "absolute",
    bottom: 40,
    left: 0,
    right: 0,
    alignItems: "center"
  },
  deleteText: {
    color: colors.GRAY_400,
    fontSize: 12
  },
  deletePressed: {
    opacity: 0.6
  }
});
