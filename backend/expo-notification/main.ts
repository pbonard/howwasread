import {
  createCassandraClient,
  createKafkaConsumer,
  createKafkaProducer,
  createValkeyClient,
  GROUP_ID,
} from "@/config";
import { extractData } from "@/util";
import { sendPushNotification } from "@/service";
import Expo from "expo-server-sdk";
import { removeNotificationInfoByIdAndToken } from "@/db";
import type { RetryEvent } from "@/types";

async function main() {
  const expo = new Expo();
  const consumer = await createKafkaConsumer();
  const producer = await createKafkaProducer();
  const valkey = await createValkeyClient();
  const cassandra = await createCassandraClient();

  const disconnect = () => {
    process.off("SIGINT", disconnect);
    process.off("SIGTERM", disconnect);
    consumer
      .commitOffsets()
      .finally(() => consumer.disconnect())
      .finally(() => producer.disconnect())
      .finally(() => valkey.close())
      .finally(() => cassandra.shutdown())
      .finally(() => console.log("Disconnected successfully"));
  };
  process.on("SIGINT", disconnect);
  process.on("SIGTERM", disconnect);

  consumer.run({
    partitionsConsumedConcurrently: 6,
    eachMessage: async ({ topic, partition, message }) => {
      const headers = message.headers ?? {};
      const partitionIdHeader = headers["partitionId"]?.toString();
      // a copy re-sent for another group's retry, this group handles the original record itself
      if (partitionIdHeader && !partitionIdHeader.startsWith(`${GROUP_ID}:`)) {
        return;
      }
      let data;
      try {
        data = extractData(message);
      } catch (error) {
        console.log("skip malformed message: ", error);
        return;
      }
      const { value, key, member } = data;
      try {
        const did = await valkey.sismember(key, member);
        if (did) {
          return;
        }
        const failedTokenMap = await sendPushNotification(
          expo,
          () => valkey.sadd(key, [member]),
          value.tokenMap,
          value.title,
          value.subTitle,
          value.text,
          value.imageURL,
        );
        if (Object.keys(failedTokenMap).length > 0) {
          await Promise.all(
            Object.entries(failedTokenMap).map(([token, id]) => {
              return removeNotificationInfoByIdAndToken(
                cassandra,
                id,
                token,
              );
            }),
          );
        }
      } catch (error) {
        console.log("Error message: ", error);
        const event: RetryEvent = {
          partitionId: "",
          reason: error instanceof Error ? error.message : String(error),
        };
        // a re-sent record of this group's retry continues the same partition id
        if (partitionIdHeader) {
          event.partitionId = partitionIdHeader;
        } else {
          event.partitionId = `${GROUP_ID}:${topic}:${partition}:${message.offset}`;
          event.backoff = Math.round(2000 * (0.9 + Math.random() * 0.2));
          event.multiplier = 2;
          event.cap = 15000000;
          event.maxFailure = 5;
          event.topic = topic;
          event.key = message.key?.toString("base64");
          event.headers = {};
          for (const [k, v] of Object.entries(headers)) {
            if (k !== "partitionId" && v !== undefined) {
              event.headers[k] = Buffer.from(
                Array.isArray(v) ? v[v.length - 1] : v,
              ).toString("base64");
            }
          }
          event.value = message.value?.toString("base64");
        }
        // a failed send throws, the offset is not committed and the message is consumed again
        await producer.send({
          topic: "exponential-backoff-retry",
          messages: [{ key: event.partitionId, value: JSON.stringify(event) }],
        });
      }
    },
  });
}

main();
