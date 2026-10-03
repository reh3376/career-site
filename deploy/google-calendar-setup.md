# Connecting Google Calendar to the meeting scheduler

A one-time setup, done once by the owner. About twenty minutes, most of
it in Google's console.

Everything else about the scheduler already works without this. Until it
is done, `/admin/scheduler` says no calendar is connected and members
see "booking is not switched on yet" rather than an empty calendar that
would read as "never available".

**Do not paste the client secret or the sealing key into a chat, an
issue, or a commit.** They go in one file on the server and nowhere
else. `.env.prod` is not in git, and gitleaks runs in CI as a backstop.

---

## Part 1: create the OAuth client (in a browser, about 10 minutes)

### 1.1 Pick or create a Google Cloud project

1. Go to https://console.cloud.google.com/ and sign in as the Google
   account whose calendar you want booked. **This matters:** the
   calendar that gets written to is the account you authorise with in
   Part 4, so sign in as yourself, not a work account.
2. Top left, click the project picker. Either choose an existing
   project or **New project**.
3. If new: name it something you will recognise later, for example
   `rogerhenley-dev`, and click **Create**. Wait for it to finish, then
   make sure the picker shows it as the active project.

### 1.2 Enable the Calendar API

1. Left menu → **APIs & Services** → **Library**.
2. Search `Google Calendar API`.
3. Click it, then **Enable**.

Nothing works without this, and the error if you skip it arrives much
later and does not mention it.

### 1.3 Configure the consent screen

Left menu → **APIs & Services** → **OAuth consent screen**.

1. **If Google asks "What data will you be accessing?", choose
   `User data`.** This fork appears on some paths through the console
   and not others; the wrong branch leads to a service-account flow that
   cannot work here. See the note below.
2. User type: **External**. Internal is only offered with Google
   Workspace, and even then External is what you want here.
3. **Create**.
4. App information:
   - App name: something you will recognise on the consent screen, for
     example `rogerhenley.dev scheduler`.
   - User support email: your own address.
   - Developer contact information: your own address.
5. **Save and continue**.
6. Scopes: **Save and continue** without adding any. The application
   asks for its scopes at the moment it sends you to Google, so
   anything added here is redundant.
7. Test users: **Add users**, enter your own Google address, **Add**,
   then **Save and continue**.
8. Summary: **Back to dashboard**.

**Publishing status: see "Publishing and the seven-day token" below.**

This document used to say "leave it as Testing, publishing triggers
Google's verification review, which is weeks of work". That is wrong
and it cost a weekly reconnect for no reason. Publishing and submitting
for verification are two separate actions: you can move to **In
production** and simply never submit. Google's own help describes
verification as a request the developer chooses to send.

**Why User data and not Application data.** The distinction is whose
data it is. User data belongs to a Google user and is reached by asking
that user's permission, which is exactly this: it is your calendar, and
you grant access on the consent screen in Part 4. Application data
belongs to the application itself and is reached with a service account
and no human consent.

Application data is not merely the wrong choice here, it is impossible.
A service account is its own identity with its own empty calendar and
cannot read yours. Making one act as you requires domain-wide
delegation, which needs a Google Workspace admin console; a personal
Google account has none, and it would be a far heavier grant than
booking a meeting needs.

### 1.4 Create the OAuth client

Left menu → **APIs & Services** → **Credentials**.

1. **Create credentials** → **OAuth client ID**.
2. Application type: **Web application**.
3. Name: anything, for example `career-site api`. Only you see it.
4. Under **Authorised redirect URIs**, click **Add URI** and paste
   exactly:

       https://rogerhenley.dev/admin/scheduler/callback

   Character for character. No trailing slash, `https` not `http`, no
   `www`. If this is wrong by one character Google refuses with
   `redirect_uri_mismatch` at the last step and nothing explains why.

   Leave **Authorised JavaScript origins** empty. This flow never runs
   in the browser.
5. **Create**.
6. A panel shows **Client ID** and **Client secret**. Copy both now.
   The secret can be re-shown later from this page, but it is easier to
   take it now.

---

## Part 2: generate the sealing key (on your Mac, 10 seconds)

The refresh token Google gives us is encrypted before it is stored. That
needs a key.

    openssl rand -base64 32

It prints one line of 44 characters ending in `=`. That whole line is
the key.

**Keep a copy somewhere safe**, a password manager is ideal. Losing it
does not lose your calendar and does not lose any booking: it loses the
stored credential. The api logs that it could not open it, the scheduler
reports not-connected, and pressing **Reconnect** stores a fresh one
under the new key. Recoverable, but a minute of confusion if you have
forgotten why.

---

## Part 3: put all three on the server (about 5 minutes)

### 3.1 Sign in and back up the current file

    ssh career@5.161.62.205
    cd /opt/career-site
    cp .env.prod .env.prod.bak.$(date +%Y%m%d%H%M%S)

### 3.2 Add the three lines

    nano .env.prod

Go to the end of the file and add:

    GOOGLE_CLIENT_ID=<paste the client id>
    GOOGLE_CLIENT_SECRET=<paste the client secret>
    SECRETS_KEY=<paste the openssl line>

Formatting rules, each of which has bitten somebody:

- **No quotes.** `SECRETS_KEY=abc=` not `SECRETS_KEY="abc="`. Docker
  Compose treats the quotes as part of the value.
- **No spaces around `=`.**
- **No trailing spaces** after the value. A trailing space becomes part
  of the secret and produces a puzzling refusal.
- The client id ends in `.apps.googleusercontent.com`. Include that.
- The sealing key ends in `=`. That is padding, keep it.

Save and exit: `Ctrl-O`, `Enter`, `Ctrl-X`.

### 3.3 Check what you typed, without printing the secrets

    grep -c '^GOOGLE_CLIENT_ID=' .env.prod
    grep -c '^GOOGLE_CLIENT_SECRET=' .env.prod
    grep -c '^SECRETS_KEY=' .env.prod

Each should print `1`. If any prints `0` the line name is wrong; if any
prints `2` you have a duplicate and the last one wins.

To check the sealing key is the right length without revealing it:

    grep '^SECRETS_KEY=' .env.prod | cut -d= -f2- | tr -d '\n' | wc -c

Should print `44`.

### 3.4 Restart the api

Only the environment changed, so the api alone needs recreating. **Use
the full invocation**, not a bare `docker compose up`:

    docker compose --env-file .env.prod \
      -f docker-compose.yml -f docker-compose.prod.yml \
      up -d api

Both flags matter, and leaving either out fails in a way that does not
mention the flag.

`--env-file .env.prod` is what makes Compose read this file at all.
Without it Compose looks for a plain `.env`, which does not exist here,
so the container starts with no configuration and cannot even reach the
database. The symptom is the api crash-looping on
`password authentication failed for user "career"` and the site
answering 502 from Caddy, which looks nothing like a missing env file.

`-f docker-compose.prod.yml` is what makes it use the image CI built.
The prod overlay clears the `build:` stanza, so without it Compose
rebuilds the api from the server's source checkout, which is slow and
leaves production running an image nobody has a record of.

### 3.5 Confirm it read them

    docker compose --env-file .env.prod \
      -f docker-compose.yml -f docker-compose.prod.yml \
      logs api --since 2m | grep -i 'secrets\|google'

**Silence is success.** The api warns only when something is missing:

- `SECRETS_KEY not set — the meeting scheduler cannot connect a calendar`
- `Google client credentials not set — booking stays switched off`

If either appears the variable did not reach the container. That is a
different failure from the value being wrong: check 3.3, confirm the
api service in `docker-compose.prod.yml` lists the variable under
`environment:`, and recreate again. A value present in `.env.prod` but
absent from the compose file is invisible to the container, because
`--env-file` feeds Compose's own substitution rather than the process.

---

## Part 4: connect (about 2 minutes)

1. Go to https://rogerhenley.dev/admin/scheduler and sign in as admin.
2. The calendar panel should now say no calendar is connected, with a
   **Connect Google Calendar** button. If it still says the deployment
   cannot connect a calendar, Part 3 did not take effect.
3. Click **Connect Google Calendar**. You go to Google's own page. It is
   deliberately not embedded in this site: a page asking for Google
   credentials inside someone else's chrome is the shape of a phishing
   page.
4. Choose the account whose calendar you want booked.
5. **You will see "Google hasn't verified this app".** Expected. It is
   the consequence of being **unverified**, not of being in Testing, so
   it stays after publishing until brand verification passes. Click
   **Advanced**, then **Go to rogerhenley.dev scheduler (unsafe)**. It
   is your own application; the warning means unreviewed, not unsafe.
6. The consent screen lists what is being asked for. It should mention
   seeing and editing **events** on your calendars, and seeing when you
   are busy. It should **not** ask to read all your calendar data.
   Click **Continue**.
7. You land back on the site, which exchanges the code and shows either
   "Calendar connected" or a reason it did not.
8. Back on `/admin/scheduler`, the panel should now name the account and
   say when Google last answered.

---

## Part 5: prove it actually works (about 5 minutes)

Connecting is not the same as working, so book something.

1. **Set your hours.** On `/admin/scheduler`, check the windows. The
   defaults are Tuesday, Wednesday and Thursday, 09:00 to 12:00 and
   14:00 to 16:00, Eastern. Adjust and **Save availability**.
2. **Look at it as a member.** Go to https://rogerhenley.dev/meetings.
   You should see real times grouped by day. If it says nothing is free,
   check your calendar is not genuinely full across those windows, and
   that the lead time (24 hours by default) is not eating the next day.
3. **Book one.** Pick a slot, type something in the topic, book it.
4. **Check Google.** Open Google Calendar. The event should be there at
   the time you picked, with your own topic text, and you should have
   an invitation by email.
5. **Check the clearance.** Go back to `/meetings` and confirm the 15
   minutes either side of your test booking are no longer offered. This
   is the part most worth seeing with your own eyes, because it is the
   rule that protects you from back-to-back meetings.
6. **Cancel it.** On `/admin/scheduler`, find it under booked meetings
   and cancel. The Google event should disappear and the slot should
   come back on `/meetings`.

If all six work, the scheduler is live.

---

## Things that go wrong, and what they mean

**`redirect_uri_mismatch` at step 4.3.** The URI in the OAuth client
does not match `https://rogerhenley.dev/admin/scheduler/callback`
exactly. Check for a trailing slash, `http`, or `www`.

**"Google returned no refresh token."** Google issues one only on a
first authorisation unless asked to force consent, which this
application does. If you see it anyway, revoke the app at
https://myaccount.google.com/permissions and connect again.

**Booking works, then stops about a week later.** Google expires refresh
tokens for projects whose publishing status is **Testing** after seven
days. Press **Reconnect** and it works for another week. The real fix is
to publish, which is the next section.

You should not find out this way. A scheduled job watches the
credential and mails you when it stops working:

- `calendar-health` runs every six hours
  (`services/api/internal/handlers/calendar_health.go`).
- Each run makes a real Google call. If it is refused, you get one email
  carrying Google's own error text and a reconnect link.
- The marker is the `app_settings` key `calendar_health_warned_for`,
  holding the connection's `updated_at`. Because the marker is the
  timestamp rather than a boolean, pressing **Reconnect** re-arms it
  automatically; there is nothing to reset by hand. A recovery clears
  it, so a second failure is reported rather than swallowed.

It reports a break rather than predicting one, within six hours. The
first version predicted instead, counting seven days from the
connection, and that only worked while the project stayed in Testing;
see "Making the warning stop being a lie" below for why it was changed.

The probe is also a keep-alive. Google expires a refresh token unused
for six months, and a credential exercised every six hours never gets
near that, so the one remaining time-based expiry defends itself.

The warning is advisory. It does not refresh anything, because only you
can complete Google's consent screen. If it fires and you ignore it,
booking stays broken exactly as described above.

---

## Publishing and the seven-day token

The seven-day expiry is a property of the **Testing** publishing status,
not of being unverified. Google's OAuth documentation ties it to "a
publishing status of 'Testing'". Leaving Testing removes it.

**Publishing is not the same as submitting for verification.** This
document asserted the opposite for weeks, which is why the reconnect was
treated as unavoidable. They are two separate actions, and Google's help
describes verification as a request the developer chooses to send. You
can publish and never submit.

### What changes, and what does not

| | Testing | In production, unverified |
|---|---|---|
| Refresh token | expires every 7 days | no 7-day expiry |
| "Google hasn't verified this app" | shown | **still shown** |
| New-user cap | 100 test users | 100 new users |
| Verification review | n/a | only if you submit one |

The user cap is irrelevant at one user. The warning screen is unchanged,
because it comes from being unverified rather than from Testing.

### Doing it

1. Google Auth Platform → **Audience** → **Publish app**, and confirm.
2. Reconnect once on `/admin/scheduler`. The existing token keeps its
   seven-day fate; a new one minted under the new status does not.

### Making the warning stop being a lie

**Done on 2026-10-03, before publishing rather than after.**

`calendar-expiry-warn` hard-coded `tokenLife = 7 * 24h` and warned two
days out. Publishing removes that deadline without removing the
arithmetic, so the job would have mailed a reconnect warning about five
days after every connection, for an expiry that no longer happens. One
per connection rather than a flood, because the marker is keyed on
`updated_at`, but a warning that is sometimes wrong is one you learn to
skip, and then it is worth less than no warning.

It now probes instead of predicting. `calendar-health` calls the
provider's `Healthy` on every run and reports what Google actually says.
There is no token lifetime left in the code, so there is nothing to go
stale when the publishing status changes: it is correct in Testing,
after publishing, and if the credential is revoked by hand.

The one place seven days survive is a hint in the email. If the
connection happens to be six to nine days old when it fails, the message
names the Testing cap as the likely cause and says publishing ends it.
That is read off the connection's age, not off an assumed status.

### The logo

Publishing does not display an app name or logo on the consent screen.
That needs **brand verification**, which is the lighter-weight process:
automated and usually a few minutes, with a manual review of two to
three business days only in some cases. Not the heavyweight security
review that applies to restricted scopes.

The logo to upload is `apps/web/public/images/oauth-logo.png`, 120 square
as Google recommends. It is the same mark as the site's favicon, a step
response settling onto setpoint, so the browser tab and the consent
screen agree about who is asking.

**"The calendar is not reachable right now."** Google answered with an
error rather than refusing the credential. Usually transient. The
booked-meetings list and the panel both show the last error and when it
last worked, which distinguishes "never worked" from "stopped on
Tuesday".

**A meeting shows "held here, but no calendar event was created".** The
slot was claimed in the database and Google then refused the event. The
time is held, so nobody else can take it, but nothing is on your
calendar. Cancel it from `/admin/scheduler` and book again.

---

## What this can and cannot see

The scopes asked for are `calendar.events` and `calendar.freebusy`. The
application can create and delete its own events, and ask when you are
busy. It **cannot** read the title, attendees, location or contents of
anything else on your calendar, and never lists your events.

That is a deliberate design decision (FR-CNT-26), not a limitation that
happened by accident, and it is why the scope is not
`calendar.readonly`: a permission not held cannot be misused by a later
change to this code.

Disconnecting, from `/admin/scheduler`, clears the stored credential and
stops new bookings. Meetings already agreed are left alone, both on your
calendar and in this application.
