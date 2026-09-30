// Video services a member may host in, and where they go to set one up.
//
// Kept out of actions.ts because that file is "use server", which may
// only export async functions: a plain array there fails the build with
// "A use server file can only export async functions, found object".
// Nothing here runs on the server, so it belongs in its own module.
//
// This application creates no rooms and holds no credentials for any of
// these. The member names the service, hosts the call, and sends the
// link. Three fewer OAuth grants and three fewer things to break.
export const VIDEO_PROVIDERS = [
  {
    value: "VIDEO_PROVIDER_GOOGLE_MEET",
    label: "Google Meet",
    setupUrl: "https://calendar.google.com/calendar/u/0/r/eventedit",
  },
  {
    value: "VIDEO_PROVIDER_TEAMS",
    label: "Microsoft Teams",
    setupUrl: "https://outlook.office.com/calendar/deeplink/compose",
  },
  {
    value: "VIDEO_PROVIDER_ZOOM",
    label: "Zoom",
    setupUrl: "https://zoom.us/meeting/schedule",
  },
] as const;
