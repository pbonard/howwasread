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
import YearInput from "@/components/conversation/YearInput";
import MonthDayInput from "@/components/conversation/MonthDayInput";
import HourInput from "@/components/conversation/HourInput";
import MinuteInput from "@/components/conversation/MinuteInput";
import LengthInput from "@/components/conversation/LengthInput";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import useKeyboard from "@/hooks/useKeyboard";
import { openBrowserAsync } from "expo-web-browser";
import { useCreateOfflineConversation } from "@/hooks/useConversation";
import CustomButton from "@/components/CustomButton";
import MapsLinkInput from "@/components/conversation/MapsLinkInput";
import LocationInput from "@/components/conversation/LocationInput";
import { reverseGeocodeAsync } from "expo-location";
import { makeTime } from "@/util/time";
import Toast from "react-native-toast-message";
import { router } from "expo-router";
import { latLngToCell } from "h3-js";
import GoogleMapsResolver from "@/components/conversation/GoogleMapsResolver";
import { useEffect, useState } from "react";

interface FormValue {
  novel?: string;
  shortStory?: string;
  poem?: string;
  play?: string;
  film?: string;
  writtenBy: string;
  description?: string;
  year: string;
  monthDay: string;
  hour: string;
  minute: string;
  length: string;
  mapsLink: string;
  location?: string;
}

export default function OfflineConversationScreen() {
  const createOfflineConversationMutation = useCreateOfflineConversation();
  const { isKeyboardVisible } = useKeyboard();
  const insets = useSafeAreaInsets();
  const now = new Date();
  const [resolvedGeoInfo, setResolvedGeoInfo] = useState<{
    lat: number;
    lng: number;
    placeName: string;
  } | null>(null);
  const offlineConversationForm = useForm<FormValue>({
    defaultValues: {
      novel: "",
      shortStory: "",
      poem: "",
      play: "",
      film: "",
      writtenBy: "",
      description: "",
      year: String(now.getFullYear()),
      monthDay: `${now.getMonth() + 1}.${now.getDate()}`,
      hour: String(now.getHours()),
      minute: String(now.getMinutes()),
      length: "100",
      mapsLink: "",
      location: "",
    },
  });
  const mapsLinkValue = offlineConversationForm.watch("mapsLink");
  useEffect(() => {
    setResolvedGeoInfo(null);
  }, [mapsLinkValue]);

  const onSubmit = async (formValues: FormValue) => {
    let {
      novel,
      shortStory,
      poem,
      play,
      film,
      writtenBy,
      description,
      year,
      monthDay,
      hour,
      minute,
      length,
      mapsLink,
      location,
    } = formValues;
    const time = makeTime(now, year, monthDay, hour, minute);
    if (resolvedGeoInfo === null) {
      Toast.show({
        type: "error",
        text1: "fail to resolve coords",
      });
      return;
    }
    const lat = resolvedGeoInfo.lat;
    const lng = resolvedGeoInfo.lng;
    location = location ? location : resolvedGeoInfo.placeName;
    const city = (
      await reverseGeocodeAsync({
        latitude: lat,
        longitude: lng,
      })
    )[0].city;
    const h3Res5 = latLngToCell(lat, lng, 5);
    const h3Res7 = latLngToCell(lat, lng, 7);
    console.log(city);
    console.log(lat);
    console.log(lng);
    console.log(location);
    console.log(h3Res5);
    console.log(h3Res7);
    createOfflineConversationMutation.mutate(
      {
        novel: novel,
        shortStory: shortStory,
        poem: poem,
        play: play,
        film: film,
        writtenBy: writtenBy,
        description: description,
        time: time,
        lengthMinutes: Number(length),
        mapsLink: mapsLink,
        location: location,
        city: city ?? "",
        lat: lat,
        lng: lng,
        h3Res5: h3Res5,
        h3Res7: h3Res7,
      },
      {
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
      },
    );
  };

  return (
    <FormProvider {...offlineConversationForm}>
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
              <Text style={styles.whenLabel}>When</Text>
              <YearInput />
              <MonthDayInput />
              <View style={styles.timeRow}>
                <HourInput />
                <Text>:</Text>
                <MinuteInput />
              </View>
              <LengthInput />
              <CustomButton
                label="Go and get Google Maps share link"
                onPress={() => openBrowserAsync("https://www.google.com/maps")}
              />
              <MapsLinkInput />
              {mapsLinkValue && !resolvedGeoInfo && (
                <GoogleMapsResolver
                  shortUrl={mapsLinkValue}
                  onGeoInfoResolved={(geoInfo) => {
                    setResolvedGeoInfo(geoInfo);
                    console.log("Headless Resolution Success:", geoInfo);
                  }}
                />
              )}
              <LocationInput />
            </View>
          </ScrollView>
          <FixedBottomCTA
            label="Create"
            onPress={offlineConversationForm.handleSubmit(onSubmit)}
            disabled={
              resolvedGeoInfo === null ||
              createOfflineConversationMutation.isPending
            }
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
