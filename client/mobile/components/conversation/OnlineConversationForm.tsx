import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { FormProvider, useForm } from "react-hook-form";
import { colors } from "@/constants";
import FixedBottomCTA from "@/components/FixedBottomCTA";
import NovelInput from "@/components/conversation/NovelInput";
import ShortStoryInput from "@/components/conversation/ShortStoryInput";
import PoemInput from "@/components/conversation/PoemInput";
import PlayInput from "@/components/conversation/PlayInput";
import FilmInput from "@/components/conversation/FilmInput";
import WrittenBy from "@/components/conversation/WrittenBy";
import DescriptionInput from "@/components/conversation/DescriptionInput";
import CapacityInput from "@/components/conversation/CapacityInput";
import YearInput from "@/components/conversation/YearInput";
import MonthDayInput from "@/components/conversation/MonthDayInput";
import HourInput from "@/components/conversation/HourInput";
import MinuteInput from "@/components/conversation/MinuteInput";
import LengthInput from "@/components/conversation/LengthInput";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import useKeyboard from "@/hooks/useKeyboard";
import { makeTime, splitTime } from "@/util/time";
import {
  CreateOnlineConversationRequest,
  OnlineConversationDetailResponse,
} from "@/types/conversation";

export interface OnlineConversationFormValue {
  novel?: string;
  shortStory?: string;
  poem?: string;
  play?: string;
  film?: string;
  writtenBy?: string;
  description?: string;
  capacity: string;
  year: string;
  monthDay: string;
  hour: string;
  minute: string;
  length: string;
}

export function emptyOnlineConversationForm(): OnlineConversationFormValue {
  return {
    novel: "",
    shortStory: "",
    poem: "",
    play: "",
    film: "",
    writtenBy: "",
    description: "",
    capacity: "6",
    ...splitTime(new Date().toISOString()),
    length: "100",
  };
}

export function toOnlineConversationForm(
  detail: OnlineConversationDetailResponse,
): OnlineConversationFormValue {
  return {
    novel: detail.novel ?? "",
    shortStory: detail.shortStory ?? "",
    poem: detail.poem ?? "",
    play: detail.play ?? "",
    film: detail.film ?? "",
    writtenBy: detail.writtenBy ?? "",
    description: detail.description ?? "",
    capacity: String(detail.capacity),
    ...splitTime(detail.time),
    length: String(detail.lengthMinutes),
  };
}

interface OnlineConversationFormProps {
  defaultValues: OnlineConversationFormValue;
  submitLabel: string;
  isPending: boolean;
  onSubmit: (request: CreateOnlineConversationRequest) => void;
}

// shared by the create and update screens, the screen decides which request it sends
export default function OnlineConversationForm({
  defaultValues,
  submitLabel,
  isPending,
  onSubmit,
}: OnlineConversationFormProps) {
  const { isKeyboardVisible } = useKeyboard();
  const insets = useSafeAreaInsets();
  const onlineConversationForm = useForm<OnlineConversationFormValue>({
    defaultValues,
  });
  const handleSubmit = (formValues: OnlineConversationFormValue) => {
    const {
      novel,
      shortStory,
      poem,
      play,
      film,
      writtenBy,
      description,
      capacity,
      year,
      monthDay,
      hour,
      minute,
      length,
    } = formValues;
    onSubmit({
      novel: novel,
      shortStory: shortStory,
      poem: poem,
      play: play,
      film: film,
      writtenBy: writtenBy,
      description: description,
      capacity: Number(capacity),
      time: makeTime(new Date(), year, monthDay, hour, minute),
      lengthMinutes: Number(length),
    });
  };

  return (
    <FormProvider {...onlineConversationForm}>
      <View style={styles.container}>
        <KeyboardAvoidingView
          contentContainerStyle={styles.awareScrollViewContainer}
          behavior="height"
          keyboardVerticalOffset={
            Platform.OS === "ios" || isKeyboardVisible ? 100 : insets.bottom
          }
        >
          <ScrollView style={{ marginBottom: 100 }}>
            <View style={styles.content}>
              <NovelInput />
              <ShortStoryInput />
              <PoemInput />
              <PlayInput />
              <FilmInput />
              <WrittenBy />
              <DescriptionInput />
              <CapacityInput />
              <Text style={styles.whenLabel}>When</Text>
              <YearInput />
              <MonthDayInput />
              <View style={styles.timeRow}>
                <HourInput />
                <Text>:</Text>
                <MinuteInput />
              </View>
              <LengthInput />
            </View>
          </ScrollView>
          <FixedBottomCTA
            label={submitLabel}
            onPress={onlineConversationForm.handleSubmit(handleSubmit)}
            disabled={isPending}
          />
        </KeyboardAvoidingView>
      </View>
    </FormProvider>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.SAND_110,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderColor: colors.GRAY_700,
  },
  content: {
    flex: 1,
    margin: 16,
    gap: 16,
    backgroundColor: colors.SAND_110,
  },
  timeRow: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
  },
  awareScrollViewContainer: {
    flex: 1,
  },
  whenLabel: {
    fontSize: 12,
    color: colors.GRAY_700,
    marginBottom: -10,
  },
});
