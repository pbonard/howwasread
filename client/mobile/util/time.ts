export function getTimestamp(id: string): string {
  if (id.length < 36 || id.charAt(14) !== "7") {
    throw new Error("Invalid UUIDv7 string provided.");
  }

  const hexTimestamp = id.substring(0, 8) + id.substring(9, 13);

  const timestampMs = parseInt(hexTimestamp, 16);

  return new Date(timestampMs).toISOString();
}

export function getLongDate(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleDateString(undefined, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

export function getHourMinute(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function formatPreviewDate(createdAt: string) {
  const date = new Date(createdAt);
  const now = new Date();

  const isToday =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate();

  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);

  const isYesterday =
    date.getFullYear() === yesterday.getFullYear() &&
    date.getMonth() === yesterday.getMonth() &&
    date.getDate() === yesterday.getDate();

  if (isToday) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  if (isYesterday) {
    return "yesterday";
  }
  return date.toLocaleDateString(undefined, {
    month: "numeric",
    day: "numeric",
  });
}

export function formatToMinuteSecond(seconds: number) {
  if (isNaN(seconds) || seconds < 0) return "00:00";
  const m = Math.floor(seconds / 60)
    .toString()
    .padStart(2, "0");
  const s = Math.floor(seconds % 60)
    .toString()
    .padStart(2, "0");
  return `${m}:${s}`;
}

export function makeTime(
  now: Date,
  year: string,
  monthDay: string,
  hour: string,
  minute: string,
) {
  const monthDayParts = monthDay.split(".");
  const month = monthDayParts[0] ?? String(now.getMonth() + 1);
  const day = monthDayParts[1] ?? String(now.getDate());
  const time = new Date(
    Number(year),
    Number(month) - 1,
    Number(day),
    Number(hour),
    Number(minute),
  ).toISOString();
  return time;
}

export function splitTime(isoString: string) {
  const date = new Date(isoString);
  return {
    year: String(date.getFullYear()),
    monthDay: `${date.getMonth() + 1}.${date.getDate()}`,
    hour: String(date.getHours()),
    minute: String(date.getMinutes()),
  };
}
