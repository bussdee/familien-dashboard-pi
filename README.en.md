<div align="center">

# 🏠 Family Dashboard

**One shared screen for everyday family life — on a device in your own home.**

Who's taking the bins out today? What do we need from the shop? When is the
parents' evening? And when is everyone actually home?

[Deutsch](README.md) · **English**

[What is it?](#what-is-it) · [Features](#features) · [Installation](#installation) · [First steps](#first-steps) · [Honest limits](#honest-limits)

</div>

---

## What is it?

A tablet on the kitchen wall. A bookmark on every phone in the family. It
shows what's on today — and whose turn it is.

The Family Dashboard is meant to **take load off parents**, not to be one
more app that needs looking after. It answers the questions that get asked
ten times a day and turns the never-ending chores into a small game that
children want to join in.

It runs on a device in your home: a Raspberry Pi, an old laptop, a NAS, your
everyday PC. **No Google account. No subscription. No data leaving the
house.**

The interface is available in **German and English**, switchable per device.
The project itself is written in German — code comments, changelog and the
detailed install guides ([INSTALL.md](INSTALL.md), [DEPLOY.md](DEPLOY.md)).
The commands in them work the same either way.

### 🔒 First things first: it's for your home network only

This project is deliberately **simple**, not **secure** in the sense of an
application that sits on the internet:

- Sign-in is a **four-digit PIN**. It stops a child from booking points as
  Dad by accident. It won't stop anyone who seriously tries.
- There is **no content encryption**, no two-factor sign-in, no access logs.
- **Don't expose it to the internet.** No port forwarding on your router. If
  you want access from outside, use your own VPN (WireGuard, for example).

---

## Features

| | |
|---|---|
| ⭐ **Chores** | Bins, dishwasher, litter tray. Every chore has an interval and a point value. Assigned *in turn*, to one person, to *everyone* or to *whoever wants it*. Done is done — no ticking it off twice on the same day. Optionally **a parent confirms** before the points are paid out. |
| 🏆 **Points & levels** | Tick something off, get points. Levels from *Rookie* to *Legend*, streaks, eleven badges, a podium. Parents can also give and deduct points by hand — for one person or several at once, with a reason — on their phone or on the wall tablet with the **parent PIN**. |
| 🎯 **Family goal** | Something everyone works towards together: pizza night, the zoo, the cinema. Every point anyone earns counts, the bar shows who contributed how much. Deductions don't count against it. |
| 🎁 **Rewards** | What the points are for: screen time, an ice cream, the film on family night. A child redeems, a parent approves. A **balance** is spent; levels and ranking stay untouched. |
| 🛒 **Shopping list** | Shared and **live**: what's ticked off in the shop disappears on every device at once. Frequently bought items are offered as suggestions, and there's a shop view sorted by aisle that keeps the screen on. |
| 🍝 **Meal plan** | What's for dinner today — and all week. Ingredients go **onto the shopping list with one tap**, without duplicates. |
| 📅 **Calendar** | Add events directly, one-off or repeating, or drop in an `.ics` file. Countdowns pick up birthdays and holidays. |
| 📝 **Notes** | Markdown, synced both ways with real files on disk. |
| 🔗 **Links** | Bookmarks with categories, private or shared with the family. |
| 🌧️ **Weather with rain times** | Not "98 % rain" but **"rain from 18:30"**, plus dry windows for a walk. Data from Open-Meteo — no account, no API key. |
| 🖼️ **Photo frame & slideshow** | Upload pictures; a full-screen mode shows photo, time, weather and what's still on today. |
| 🎵 **Music & audiobooks** | Your own folder of MP3s, browsed by folder. Playback continues while you switch pages. |
| 📎 **Files** | Manuals, school letters, forms. A parent uploads, everyone downloads. |
| 🕗 **Work & school** | Who is away when — and from that, **when everyone is home**. |
| 🏠 **Family mode for the wall tablet** | A tablet in the hallway becomes the family device: always signed in, but nothing personal on it. Whoever ticks off a chore taps their face — **no PIN** — and the points go to the right person. Giving points and confirming chores asks for a parent's PIN, without anyone staying signed in. |
| 📱 **Device status** | Is Plex, Kavita, the NAS up? Tap the tile to open it. |
| 🇬🇧 **German or English** | Per device, including dates, weather, levels and messages. What you enter stays as you wrote it. |
| 📲 **Like an app** | Add it to your home screen: own icon, no browser bar, offline view. |

What changed in each version, and why: [CHANGELOG.md](CHANGELOG.md) (German).

---

## Installation

You'll need a device that stays on, **2 GB of RAM** recommended (1 GB works,
see the German README), about 2 GB of disk space and **Docker**.

On a Raspberry Pi (Pi 4 or 5, Raspberry Pi OS 64-bit) or any Linux machine:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# sign out and back in once, then:
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

Then open `http://<address-of-the-device>:8088` in a browser
(`hostname -I` shows the address).

> The first start builds the application and takes **10–20 minutes**. After
> that it starts in seconds.

Windows (Docker Desktop + WSL 2), Mac and NAS work too — the steps are in the
[German README](README.md#installation); the commands are identical.

---

## First steps

```bash
make verify
```

Expected: **74 checks passed.**

Open the browser and sign in. Three users exist — **Papa**, **Mama**,
**Kind** — all with the PIN `1234`. Then:

1. **Change the PINs.** Settings → Change PIN, for each person.
2. **Switch the language** if you like: Settings → Language. It applies to
   this device only.
3. **Adjust profiles and people.** Settings → My profile; Admin → Family
   members.
4. **Set the weather location.** Tap the weather, search, select.
5. **Replace the sample chores** with your own, and tick *Parents approve*
   where a parent should take a look first.

### Putting a tablet on the wall

Sign in on the tablet once as an admin, then **Admin → Wall tablet → Set up
this device as a wall tablet**. The tablet signs you out and is the family
device from now on.

When someone ticks off a chore it asks **"Who did it?"** and shows the faces.
For everything parents decide — giving or deducting points, confirming a
finished chore — it asks for a parent: tap the face, enter the PIN. The
approval lasts two minutes and is shown in the top bar; tap it to end it
early. Nobody gets signed in.

---

## Honest limits

- **No real access protection.** A four-digit PIN, nothing more.
- **Ticking off chores on the wall tablet needs no PIN.** That's on purpose:
  a child should be able to tick something off in passing. Where that's too
  generous, give the chore *Parents approve*.
- **Offline mode and app installation need a certificate the device
  trusts.** `bash scripts/make-cert.sh <your-address>` creates one.
- **Weather needs internet.** Without it, the last known forecast is shown.
- **Music never starts by itself.** Browsers don't allow sound without a tap.
- **Rewards aren't redeemed automatically.** A parent approves each request.
- **Two languages.** Adding another one is described in
  [CONTRIBUTING.md](CONTRIBUTING.md#übersetzungen) (German). What you enter
  yourself isn't translated.

---

## Technical

Go backend with SQLite (pure Go, no CGO), SvelteKit frontend as a single-page
app, Traefik as reverse proxy. **Three containers, about 150 MB of RAM.**
PINs hashed with argon2id, lockout after five failed attempts, JWT in an
HttpOnly cookie, containers as non-root with a read-only filesystem.

```bash
make check     # build, vet, type check, translation check
make up && make verify
```

## Licence

[MIT](LICENSE).
