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

1. User type: **External**. Internal is only offered with Google
   Workspace, and even then External is what you want here.
2. **Create**.
3. App information:
   - App name: something you will recognise on the consent screen, for
     example `rogerhenley.dev scheduler`.
   - User support email: your own address.
   - Developer contact information: your own address.
4. **Save and continue**.
5. Scopes: **Save and continue** without adding any. The application
   asks for its scopes at the moment it sends you to Google, so
   anything added here is redundant.
6. Test users: **Add users**, enter your own Google address, **Add**,
   then **Save and continue**.
7. Summary: **Back to dashboard**.

Leave the publishing status as **Testing**. Publishing triggers
Google's verification review, which is weeks of work for an
application with exactly one user. See the seven-day note at the end of
this document for the one cost of staying in Testing.

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

Only the environment changed, so the api alone needs recreating:

    docker compose up -d api

### 3.5 Confirm it read them

    docker compose logs api --since 2m | grep -i 'secrets\|google\|scheduler'

**Silence is success.** The api warns only when something is missing:

- `SECRETS_KEY not set — the meeting scheduler cannot connect a calendar`
- `Google client credentials not set — booking stays switched off`

If either appears, the variable did not reach the container. Re-check
3.3, then `docker compose up -d api` again.

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
5. **You will see "Google hasn't verified this app".** This is expected
   and is the consequence of staying in Testing. Click **Advanced**,
   then **Go to rogerhenley.dev scheduler (unsafe)**. It is your own
   application; the warning means unreviewed, not unsafe.
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

**Booking works, then stops about a week later.** This is the one real
cost of staying in Testing: Google expires refresh tokens for
unverified apps after **seven days**. The scheduler will say the
credential was rejected. Press **Reconnect** and it works again for
another week. To remove the limit, publish the consent screen and go
through Google's verification.

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
