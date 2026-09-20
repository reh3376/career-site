// The /home sign-out button uses the shared action so all sign-out
// paths (hamburger, member home, future admin surface) go through one
// place. Kept as a re-export so the callsite import stays local.
export { signOutAction as logoutAction } from "@/app/actions/session";
