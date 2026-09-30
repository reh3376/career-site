"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Mirrors career.v1.SchedulerWindow. Minutes from local midnight, not a
// clock string, so a window survives a daylight-saving change beneath
// it instead of drifting an hour.
export type Window = {
  weekday: number;
  startMinutes: number;
  endMinutes: number;
};

// Mirrors career.v1.SchedulerSettings.
export type SchedulerSettings = {
  zone: string;
  durationMinutes: number[];
  gapMinutes: number;
  stepMinutes: number;
  maxPerDay: number;
  leadHours: number;
  horizonDays: number;
  windows: Window[];
};

export type SchedulerState = {
  settings: SchedulerSettings;
  calendarConnected: boolean;
  calendarStatus: string;
};

export type SaveState = {
  saved?: boolean;
  // The validator's own message, which names the field. "Invalid
  // settings" would leave the owner guessing which of nine numbers.
  error?: string;
};

const EMPTY: SchedulerSettings = {
  zone: "",
  durationMinutes: [],
  gapMinutes: 0,
  stepMinutes: 0,
  maxPerDay: 0,
  leadHours: 0,
  horizonDays: 0,
  windows: [],
};

export async function getSchedulerSettings(): Promise<SchedulerState> {
  const cookie = await getSessionCookie();
  const fallback: SchedulerState = {
    settings: EMPTY,
    calendarConnected: false,
    calendarStatus: "Settings could not be loaded.",
  };
  if (!cookie) return fallback;

  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetSchedulerSettings",
    body: {},
    cookie,
  });
  if (!resp.ok) return fallback;

  const data = (await resp.json()) as Partial<SchedulerState> & {
    settings?: Partial<SchedulerSettings>;
  };
  return {
    settings: { ...EMPTY, ...data.settings,
      durationMinutes: data.settings?.durationMinutes ?? [],
      windows: data.settings?.windows ?? [],
    },
    calendarConnected: data.calendarConnected ?? false,
    calendarStatus: data.calendarStatus ?? "",
  };
}

export async function saveSchedulerSettings(
  _prev: SaveState,
  formData: FormData,
): Promise<SaveState> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Sign in again." };

  // The form serialises the whole settings object, because the server
  // validates and stores it whole: a half-applied calendar is worse
  // than an unchanged one.
  const raw = String(formData.get("settings") ?? "");
  let settings: SchedulerSettings;
  try {
    settings = JSON.parse(raw) as SchedulerSettings;
  } catch {
    return { error: "The form could not be read. Reload and try again." };
  }

  const resp = await callApi({
    path: "/api/career.v1.AdminService/SetSchedulerSettings",
    body: { settings },
    cookie,
  });
  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { message?: string };
    return { error: body.message || "The settings were refused." };
  }
  revalidatePath("/admin/scheduler");
  return { saved: true };
}

// Mirrors career.v1.CalendarStatus.
export type CalendarStatus = {
  configured: boolean;
  connected: boolean;
  accountEmail: string;
  calendarId: string;
  scopes: string;
  connectedAt?: string;
  lastOkAt?: string;
  lastError: string;
};

const NO_CALENDAR: CalendarStatus = {
  configured: false,
  connected: false,
  accountEmail: "",
  calendarId: "",
  scopes: "",
  lastError: "",
};

export async function getCalendarStatus(): Promise<CalendarStatus> {
  const cookie = await getSessionCookie();
  if (!cookie) return NO_CALENDAR;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetCalendarStatus",
    body: {},
    cookie,
  });
  if (!resp.ok) return NO_CALENDAR;
  const data = (await resp.json()) as { status?: Partial<CalendarStatus> };
  return { ...NO_CALENDAR, ...data.status };
}

// Returns the Google consent URL. The client secret never reaches the
// browser: only the id, in a URL the server built.
export async function getConnectUrl(): Promise<{ url?: string; error?: string }> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Sign in again." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetCalendarConnectURL",
    body: {},
    cookie,
  });
  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { message?: string };
    return { error: body.message || "A connect link could not be built." };
  }
  const data = (await resp.json()) as { url?: string };
  return { url: data.url };
}

// Completes the handshake. The code is single-use, so this runs once
// per callback and any error is final for that attempt.
export async function connectCalendar(
  code: string,
  state: string,
): Promise<{ ok: boolean; error?: string }> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Sign in again." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ConnectCalendar",
    body: { code, state },
    cookie,
  });
  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { message?: string };
    return { ok: false, error: body.message || "The calendar could not be connected." };
  }
  revalidatePath("/admin/scheduler");
  return { ok: true };
}

export async function disconnectCalendar(): Promise<{ ok: boolean; error?: string }> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Sign in again." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/DisconnectCalendar",
    body: {},
    cookie,
  });
  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { message?: string };
    return { ok: false, error: body.message || "The calendar could not be disconnected." };
  }
  revalidatePath("/admin/scheduler");
  return { ok: true };
}
