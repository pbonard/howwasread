import { Pressable, StyleSheet, Text, View } from "react-native";
import { router, useLocalSearchParams, useNavigation } from "expo-router";
import {
  colors,
  queryKey,
  SEAT_COORDINATES,
  SEAT_FILL_ORDER,
} from "@/constants";
import { useEffect, useRef, useState } from "react";
import {
  ConversationSignalResponse,
  SeatAssignment,
} from "@/types/conversation";
import {
  mediaDevices,
  MediaStream,
  RTCIceCandidate,
  RTCPeerConnection,
  RTCSessionDescription,
} from "react-native-webrtc";
import { getKVStore, getSecureAsync } from "@/db/storage";
import { SafeAreaView } from "react-native-safe-area-context";
import OnlineConversationRoomHeader from "@/components/conversation/OnlineConversationRoomHeader";
import { Feather, Ionicons } from "@expo/vector-icons";
import { useGetMyProfile } from "@/hooks/useProfile";
import { useActionSheet } from "@expo/react-native-action-sheet";
import { useSendMessage } from "@/hooks/useChat";
import Toast from "react-native-toast-message";
import CustomButton from "@/components/CustomButton";
import queryClient from "@/api/queryClient";
import {
  useBanParticipant,
  useGetOnlineConversationDetail,
  useGetTurn,
} from "@/hooks/useConversation";
import VoiceInput from "@/components/chat/VoiceInput";
import { RecordingPresets } from "expo-audio";

declare const WebSocket: {
  prototype: WebSocket;
  new (
    url: string,
    protocols?: string | string[] | null,
    options?: {
      headers?: { [header: string]: string };
      [key: string]: any;
    } | null,
  ): WebSocket;
};

export default function OnlineConversationScreen() {
  const { data: profile } = useGetMyProfile();
  const { id: conversationId, isPersonal } = useLocalSearchParams();
  const { data: detail } = useGetOnlineConversationDetail({
    id: String(conversationId),
    isPersonal: Boolean(isPersonal),
  });
  const { showActionSheetWithOptions } = useActionSheet();
  const sendMessageMutation = useSendMessage();
  const banParticipantMutation = useBanParticipant();
  const navigation = useNavigation();
  const { data: turn } = useGetTurn();

  const [seatAssignments, setSeatAssignments] = useState<SeatAssignment[]>([]);
  const [mute, setMute] = useState<boolean>(false);
  const participantIds = useRef<string[]>([]);
  const participantNames = useRef<Record<string, string>>({});
  const participantMutes = useRef<Record<string, boolean>>({});
  const ws = useRef<WebSocket>(null);
  const peers = useRef<Record<string, RTCPeerConnection>>({});
  const localAudio = useRef<MediaStream>(null);
  const remoteAudios = useRef<Record<string, MediaStream>>({});
  const [isWebSocketOpen, setIsWebSocketOpen] = useState(false);
  const [isVoice, setIsVoice] = useState(false);

  const coordinates = SEAT_COORDINATES[Number(detail?.capacity ?? 2)];
  const fillOrder = SEAT_FILL_ORDER[Number(detail?.capacity ?? 2)];

  const coordinateSeat = () => {
    const unique = [...new Set(participantIds.current)].sort((a, b) =>
      a.localeCompare(b),
    );
    const occupantBySeat: Record<number, string> = {};
    unique.forEach((id, idx) => {
      const seatIndex = fillOrder[idx];
      occupantBySeat[seatIndex] = id;
    });

    const nextAssignments: SeatAssignment[] = coordinates.map(
      (coordinate, idx) => ({
        id: occupantBySeat[idx],
        name: participantNames.current[occupantBySeat[idx]],
        mute: participantMutes.current[occupantBySeat[idx]],
        ...coordinate,
      }),
    );
    setSeatAssignments(nextAssignments);
  };

  const toggleAudio = () => {
    if (!profile) {
      return;
    }
    try {
      if (localAudio.current && localAudio.current.active) {
        const audioTracks = localAudio.current.getAudioTracks();
        if (
          ws.current &&
          ws.current.readyState === 1 &&
          audioTracks.length > 0
        ) {
          const audioTrack = audioTracks[0];
          audioTrack.enabled = !audioTrack.enabled;
          participantMutes.current[profile.id] =
            !participantMutes.current[profile.id];
          ws.current.send(
            JSON.stringify({
              toIds: Object.keys(peers.current),
              signal: { type: "mute" },
            }),
          );
          coordinateSeat();
          setMute(!mute);
          return;
        }
      }
    } catch (error) {
      console.error("Failed to toggle audio:", error);
    }
  };

  const handlePressParticipant = (id: string, name: string) => {
    if (detail?.isModerator) {
      showActionSheetWithOptions(
        {
          options: [`Send like to ${name}`, `Ban ${name}`, "Cancel"],
          destructiveButtonIndex: 1,
          cancelButtonIndex: 2,
        },
        (selectedIndex?: number) => {
          switch (selectedIndex) {
            case 0:
              sendMessageMutation.mutate(
                {
                  toIdType: "personal",
                  toId: id,
                  contentType: "text",
                  contents: ["👍"],
                },
                {
                  onSuccess: () => {
                    Toast.show({
                      type: "success",
                      text1: `you sent like to ${name}`,
                    });
                  },
                },
              );
              break;
            case 1:
              banParticipantMutation.mutate({
                conversationId: String(conversationId),
                banId: id,
              });
              ws.current?.send(
                JSON.stringify({
                  toIds: [id],
                  signal: { type: "ban" },
                }),
              );
              break;
            case 2:
              break;
          }
        },
      );
      return;
    }
    showActionSheetWithOptions(
      {
        options: [`Send like to ${name}`, "Cancel"],
        cancelButtonIndex: 1,
      },
      (selectedIndex?: number) => {
        switch (selectedIndex) {
          case 0:
            sendMessageMutation.mutate(
              {
                toIdType: "personal",
                toId: id,
                contentType: "text",
                contents: ["👍"],
              },
              {
                onSuccess: () => {
                  Toast.show({
                    type: "success",
                    text1: `You send like to ${name}`,
                  });
                },
              },
            );
            break;
          case 1:
            break;
        }
      },
    );
  };

  useEffect(() => {
    const joinConversation = async () => {
      ws.current = new WebSocket(
        `wss://${process.env.EXPO_PUBLIC_API_URL}/onlineconversation/join?id=${conversationId}`,
        undefined,
        {
          headers: {
            Authorization: `Bearer ${await getSecureAsync("accessToken")}`,
          },
        },
      );
      ws.current.onopen = () => {
        setIsWebSocketOpen(true);
        setIsVoice(true);
      };
      console.log(
        `wss://${process.env.EXPO_PUBLIC_API_URL}/onlineconversation/join?id=${conversationId}`,
      );
      localAudio.current = await mediaDevices.getUserMedia({
        audio: true,
        video: false,
      });
      participantIds.current = [profile?.id ?? getKVStore("myId")];
      participantNames.current[profile?.id ?? getKVStore("myId")] =
        profile?.name ?? getKVStore("myName");
      participantMutes.current[profile?.id ?? getKVStore("myName")] = false;
      coordinateSeat();

      ws.current.onmessage = async (event) => {
        console.log("get message");
        const data: ConversationSignalResponse = JSON.parse(event.data);
        if (!data.signal) {
          const unique = [...new Set(data.fromIds)];
          if (unique.length >= Number(detail?.capacity ?? 2)) {
            router.replace("/conversations");
            return;
          }
          participantIds.current = [...participantIds.current, ...unique];

          for (const fromId of unique) {
            participantMutes.current[fromId] = false;
            peers.current[fromId] = new RTCPeerConnection({
              iceServers: [
                { urls: "stun:stun.l.google.com:19302" },
                {
                  urls: turn?.uris,
                  username: turn?.username,
                  credential: turn?.credential,
                },
              ],
            });
            localAudio.current?.getTracks().forEach((track) => {
              if (localAudio.current) {
                peers.current[fromId].addTrack(track, localAudio.current);
              }
            });

            peers.current[fromId].addEventListener(
              "icecandidate",
              (event: any) => {
                if (event.candidate) {
                  ws.current?.send(
                    JSON.stringify({
                      toIds: [fromId],
                      signal: { type: "candidate", candidate: event.candidate },
                    }),
                  );
                }
              },
            );

            peers.current[fromId].addEventListener(
              "iceconnectionstatechange",
              () => {
                const peer = peers.current[fromId];
                if (!peer) {
                  return;
                }
                const state = peers.current[fromId].iceConnectionState;
                console.log(`Peer ${fromId} ICE connection state: ${state}`);
                if (
                  state === "disconnected" ||
                  state === "failed" ||
                  state === "closed"
                ) {
                  if (peers.current[fromId]) {
                    peers.current[fromId].close();
                    delete peers.current[fromId];
                  }

                  if (remoteAudios.current[fromId]) {
                    remoteAudios.current[fromId].release();
                    delete remoteAudios.current[fromId];
                  }

                  participantIds.current = participantIds.current.filter(
                    (x) => x !== fromId,
                  );
                  delete participantNames.current[fromId];
                  delete participantMutes.current[fromId];
                }
                coordinateSeat();
              },
            );

            peers.current[fromId].addEventListener("track", (event: any) => {
              if (!remoteAudios.current[fromId]) {
                remoteAudios.current[fromId] = new MediaStream();
              }
              if (event.track) {
                remoteAudios.current[fromId].addTrack(event.track);
              }
            });

            const myId = profile?.id ?? getKVStore("myId");
            const shouldICreateOffer = myId > fromId;

            if (shouldICreateOffer) {
              const offer = await peers.current[fromId].createOffer({
                offerToReceiveAudio: true,
                offerToReceiveVideo: false,
                voiceActivityDetection: true,
              });
              await peers.current[fromId].setLocalDescription(offer);
              ws.current?.send(
                JSON.stringify({
                  toIds: [fromId],
                  signal: peers.current[fromId].localDescription,
                }),
              );
            }

            ws.current?.send(
              JSON.stringify({
                toIds: [fromId],
                signal: {
                  type: "name-offer",
                  name: profile?.name ?? getKVStore("myName"),
                },
              }),
            );
          }
          coordinateSeat();
          return;
        }
        const fromId = data.fromIds[0];
        if (data.signal.type === "offer") {
          const offer = new RTCSessionDescription(data.signal);
          await peers.current[fromId].setRemoteDescription(offer);
          const answer = await peers.current[fromId].createAnswer();
          await peers.current[fromId].setLocalDescription(answer);
          ws.current?.send(
            JSON.stringify({
              toIds: [fromId],
              signal: peers.current[fromId].localDescription,
            }),
          );
          return;
        }
        if (data.signal.type === "name-offer") {
          participantNames.current = {
            ...participantNames.current,
            [fromId]: data.signal.name,
          };
          ws.current?.send(
            JSON.stringify({
              toIds: [fromId],
              signal: {
                type: "name-answer",
                name: profile?.name ?? getKVStore("myName"),
              },
            }),
          );
          coordinateSeat();
          return;
        }
        if (data.signal.type === "name-answer") {
          participantNames.current = {
            ...participantNames.current,
            [fromId]: data.signal.name,
          };
          coordinateSeat();
          return;
        }
        if (data.signal.type === "answer") {
          const offerDescription = new RTCSessionDescription(data.signal);
          await peers.current[fromId].setRemoteDescription(offerDescription);
          return;
        }
        if (data.signal.type === "candidate") {
          const iceCandidate = new RTCIceCandidate(data.signal.candidate);
          await peers.current[fromId].addIceCandidate(iceCandidate);
          return;
        }
        if (data.signal.type === "ban") {
          await queryClient.invalidateQueries({
            queryKey: [
              queryKey.CONVERSATION,
              queryKey.GET_ONLINE_CONVERSATIONS,
            ],
          });
          await queryClient.invalidateQueries({
            queryKey: [
              queryKey.CONVERSATION,
              queryKey.GET_OFFLINE_CONVERSATION_DETAIL,
              conversationId,
            ],
          });
          router.replace("/conversations");
        }
        if (data.signal.type === "mute") {
          participantMutes.current[fromId] = !participantMutes.current[fromId];
          coordinateSeat();
        }
        if (data.signal.type === "leave") {
          peers.current[fromId].close();
          delete peers.current[fromId];
          remoteAudios.current[fromId].release();
          delete remoteAudios.current[fromId];
          participantIds.current = participantIds.current.filter(
            (x) => x !== fromId,
          );
          delete participantNames.current[fromId];
          coordinateSeat();
        }
      };
    };
    joinConversation();

    return () => {
      if (ws.current && ws.current.readyState === 1) {
        ws.current?.send(
          JSON.stringify({
            toIds: Object.keys(peers.current),
            signal: { type: "leave" },
          }),
        );
      }
      for (const peer of Object.values(peers.current)) {
        peer.close();
      }
      localAudio.current?.release();
      for (const remoteAudio of Object.values(remoteAudios.current)) {
        remoteAudio.release();
      }
      ws.current?.close();
    };
  }, []);

  useEffect(() => {
    if (!isPersonal) {
      return;
    }
    navigation.setOptions({
      title: "Voice call",
      headerShown: true,
      headerLeft: () => (
        <Pressable onPress={() => router.back()}>
          <Ionicons name="chevron-back" size={28} color="black" />
        </Pressable>
      ),
    });
  }, [isPersonal]);

  if (!profile || (!isPersonal && !detail)) {
    return null;
  }

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.hiddenContainer}>
        {isVoice && (
          <VoiceInput
            recordingPresets={RecordingPresets.LOW_QUALITY}
            setIsVoice={setIsVoice}
            handleFileMessage={async (contentType, mimeType, content) => {}}
          />
        )}
      </View>
      {!isPersonal && (
        <OnlineConversationRoomHeader
          novel={String(detail?.novel ?? "")}
          shortStory={String(detail?.shortStory ?? "")}
          poem={String(detail?.poem ?? "")}
          play={String(detail?.play ?? "")}
          film={String(detail?.film ?? "")}
          writtenBy={String(detail?.writtenBy ?? "")}
          description={String(detail?.description ?? "")}
          time={String(detail?.time ?? "")}
          length={String(detail?.lengthMinutes ?? "") + "m"}
        />
      )}
      <View style={styles.participantContainer}>
        <View style={styles.participantArea}>
          {seatAssignments.map((seat, idx) => (
            <View
              key={idx}
              style={[
                styles.participantSeat,
                { left: `${seat.left}%`, top: `${seat.top}%` },
              ]}
            >
              {seat.id ? (
                <>
                  {seat.mute ? (
                    <Feather name="mic-off" size={10} color="black" />
                  ) : null}
                  <Ionicons
                    name="person-circle-outline"
                    size={24}
                    color="black"
                    onPress={
                      seat.id !== profile.id
                        ? () =>
                            handlePressParticipant(
                              seat.id ?? "",
                              seat.name ?? "",
                            )
                        : undefined
                    }
                  />
                </>
              ) : (
                <Feather name="circle" size={24} color="black" />
              )}
              <Text style={styles.participantId}>{seat.name ?? ""}</Text>
            </View>
          ))}
        </View>
        <View style={styles.controls}>
          {!isWebSocketOpen ? (
            <CustomButton label="Connecting..." disabled={true} />
          ) : mute ? (
            <CustomButton label="Turn on your mic" onPress={toggleAudio} />
          ) : (
            <CustomButton label="Turn off your mic" onPress={toggleAudio} />
          )}
        </View>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.SAND_110,
  },
  content: {
    flex: 1,
    padding: 16,
  },
  title: {
    fontSize: 18,
    color: colors.BLACK,
    fontWeight: 600,
    marginVertical: 8,
  },
  description: {
    fontSize: 16,
    color: colors.BLACK,
    marginBottom: 14,
  },
  participantContainer: {
    flex: 1,
  },
  participantArea: {
    flex: 1,
    position: "relative",
  },
  participantSeat: {
    position: "absolute",
    width: 96,
    alignItems: "center",
    transform: [{ translateX: -48 }, { translateY: -12 }],
  },
  participantId: {
    marginTop: 6,
    fontSize: 12,
    color: colors.BLACK,
  },
  controls: {
    flexDirection: "row",
    justifyContent: "center",
    alignItems: "center",
    paddingVertical: 24,
    paddingHorizontal: 16,
    gap: 12,
  },
  hiddenContainer: {
    width: 0,
    height: 0,
    opacity: 0,
    position: "absolute",
  },
});
