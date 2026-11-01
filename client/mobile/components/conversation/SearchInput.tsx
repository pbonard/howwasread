import { StyleSheet, TextInput, TextInputProps, View } from "react-native";
import { colors } from "@/constants";
import Ionicons from "@expo/vector-icons/Ionicons";
import { GlassView, isLiquidGlassAvailable } from "expo-glass-effect";
import { ReactNode } from "react";

// liquid glass only exists on ios 26+, elsewhere the box is the plain white pill
const hasLiquidGlass = isLiquidGlassAvailable();

interface SearchInputProps extends TextInputProps {
  onSubmit?: () => void;
  submitKeyword: string;
  keyword: string;
  onCancel: () => void;
}

export default function SearchInput({
  onSubmit,
  submitKeyword,
  keyword,
  onCancel,
  ...props
}: SearchInputProps) {
  return (
    <Container>
      <TextInput
        style={styles.input}
        autoCapitalize="none"
        placeholderTextColor={colors.GRAY_500}
        returnKeyType="search"
        onSubmitEditing={onSubmit}
        {...props}
      />
      {keyword.length > 0 && submitKeyword === keyword ? (
        <Ionicons name="close" size={20} onPress={() => onCancel()} />
      ) : (
        <Ionicons
          name="search"
          size={20}
          onPress={props.onPress ?? onSubmit}
          color={colors.GRAY_500}
        />
      )}
    </Container>
  );
}

function Container({ children }: { children: ReactNode }) {
  if (hasLiquidGlass) {
    // light like the tab bar, the glass would turn dark over the dark map
    return (
      <GlassView style={styles.container} colorScheme="light" isInteractive>
        {children}
      </GlassView>
    );
  }
  return <View style={[styles.container, styles.fallback]}>{children}</View>;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    gap: 8,
    flexDirection: "row",
    alignItems: "center",
    paddingVertical: 8,
    height: 42,
    paddingRight: 13,
    borderRadius: 40,
  },
  fallback: {
    backgroundColor: colors.WHITE,
  },
  input: {
    flex: 1,
    fontSize: 17,
    paddingVertical: 0,
    paddingLeft: 20,
    color: colors.BLACK,
    fontFamily: "SpaceMono",
  },
});
