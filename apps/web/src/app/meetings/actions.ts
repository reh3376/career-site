"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Mirrors career.v1.GetMeetingOptionsResponse. Everything the form is
// built from comes from the server, so the page cannot disagree with
// the owner's live settings about what is on offer.
export type MeetingOptions = {
  durationMinutes: number[];
  zone: string;
  zoneLabel: string;
  horizonDays: number;
  leadHours: number;
  hoursSummary: string;
  available: boolean;
  unavailableReason: string;
};

// Mirrors career.v1.MeetingSlot. Instants, rendered in the owner's
// zone; never converted to the visitor's.
export type Slot = { start: string; end: string };

// Mirrors career.v1.Meeting.
export type Meeting = {
  id: string;
  start: string;
  end: string;
  durationMinutes: number;
  topic: string;
  zone: string;
  icsUrl: string;
  cancelledAt?: string;
  // How the meeting happens. Absent on bookings made before this was
  // asked, which is a real state rather than a gap to paper over.
  meetingType?: "MEETING_TYPE_VIDEO" | "MEETING_TYPE_PHONE" | "MEETING_TYPE_UNSPECIFIED";
  videoProvider?:
    | "VIDEO_PROVIDER_GOOGLE_MEET"
    | "VIDEO_PROVIDER_TEAMS"
    | "VIDEO_PROVIDER_ZOOM"
    | "VIDEO_PROVIDER_UNSPECIFIED";
  phoneNumber?: string;
};


export type AvailabilityState = {
  slots: Slot[];
  zone: string;
  available: boolean;
  unavailableReason: string;
};

export type BookState = {
  ok?: boolean;
  meeting?: Meeting;
  error?: string;
  // Set when the slot went while the member was deciding, so the page
  // knows to refetch rather than just showing a message about a list it
  // is still displaying.
  stale?: boolean;
};

const DEFAULT_OPTIONS: MeetingOptions = {
  durationMinutes: [],
  zone: "America/New_York",
  zoneLabel: "Eastern time",
  horizonDays: 0,
  leadHours: 0,
  hoursSummary: "",
  available: false,
  unavailableReason: "Booking is unavailable right now.",
};

export async function getMeetingOptions(): Promise<MeetingOptions> {
  const cookie = await getSessionCookie();
  if (!cookie) return DEFAULT_OPTIONS;
  const resp = await callApi({
    path: "/api/career.v1.MeetingService/GetMeetingOptions",
    body: {},
    cookie,
  });
  if (!resp.ok) return DEFAULT_OPTIONS;
  const data = (await resp.json()) as Partial<MeetingOptions>;
  return { ...DEFAULT_OPTIONS, ...data, durationMinutes: data.durationMinutes ?? [] };
}

// Availability is fetched per meeting length, because a 45-minute
// meeting has fewer places to go than a 15-minute one.
export async function getAvailability(durationMinutes: number): Promise<AvailabilityState> {
  const cookie = await getSessionCookie();
  if (!cookie) {
    return { slots: [], zone: "", available: false, unavailableReason: "Sign in to see availability." };
  }
  const resp = await callApi({
    path: "/api/career.v1.MeetingService/GetAvailability",
    body: { durationMinutes },
    cookie,
  });
  if (!resp.ok) {
    return {
      slots: [],
      zone: "",
      available: false,
      unavailableReason: "Availability could not be loaded. Please try again shortly.",
    };
  }
  const data = (await resp.json()) as Partial<AvailabilityState>;
  return {
    slots: data.slots ?? [],
    zone: data.zone ?? "",
    // The server distinguishes "nothing is free" from "we do not know",
    // and the page must not render those the same way.
    available: data.available ?? false,
    unavailableReason: data.unavailableReason ?? "",
  };
}

export async function listMyMeetings(includePast = false): Promise<Meeting[]> {
  const cookie = await getSessionCookie();
  if (!cookie) return [];
  const resp = await callApi({
    path: "/api/career.v1.MeetingService/ListMyMeetings",
    body: { includePast },
    cookie,
  });
  if (!resp.ok) return [];
  const data = (await resp.json()) as { meetings?: Meeting[] };
  return data.meetings ?? [];
}

export async function bookMeetingAction(
  _prev: BookState,
  formData: FormData,
): Promise<BookState> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Sign in to book a meeting." };

  const start = String(formData.get("start") ?? "");
  const durationMinutes = Number(formData.get("duration_minutes") ?? 0);
  const topic = String(formData.get("topic") ?? "").trim();
  const contactPreference = String(formData.get("contact_preference") ?? "").trim();
  const meetingType = String(formData.get("meeting_type") ?? "");
  const videoProvider = String(formData.get("video_provider") ?? "");
  const phoneNumber = String(formData.get("phone_number") ?? "").trim();

  if (!start || !durationMinutes) {
    return { error: "Pick a time before booking." };
  }
  // Asked for rather than optional: a meeting with no subject is one
  // Roger arrives at cold, and it is the only thing he gets from this
  // form that he cannot get from the calendar entry itself.
  if (topic.length < 10) {
    return { error: "Say a little about what you would like to discuss, so Roger can prepare." };
  }

  const resp = await callApi({
    path: "/api/career.v1.MeetingService/BookMeeting",
    body: {
      start,
      durationMinutes,
      topic,
      contactPreference,
      meetingType,
      videoProvider: videoProvider || undefined,
      phoneNumber,
    },
    cookie,
  });

  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { code?: string; message?: string };
    if (body.code === "already_exists") {
      return {
        error: "That time was taken while you were deciding. Here are the times still free.",
        stale: true,
      };
    }
    if (body.code === "failed_precondition") {
      return { error: "Booking is not available at the moment." };
    }
    return { error: body.message || "The meeting could not be booked." };
  }

  const data = (await resp.json()) as { meeting?: Meeting };
  revalidatePath("/meetings");
  return { ok: true, meeting: data.meeting };
}

export async function cancelMeetingAction(id: string): Promise<{ ok: boolean; error?: string }> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Sign in to cancel." };
  const resp = await callApi({
    path: "/api/career.v1.MeetingService/CancelMeeting",
    body: { id },
    cookie,
  });
  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { message?: string };
    return { ok: false, error: body.message || "The meeting could not be cancelled." };
  }
  revalidatePath("/meetings");
  return { ok: true };
}
