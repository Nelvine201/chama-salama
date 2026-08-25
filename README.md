# Chama Salama

**An offline-first savings group management system, built for how chamas actually meet in Kenya.**

**Live landing page:** https://nelvine201.github.io/chama-salama/

---

## The Problem

Chamas — informal savings and investment groups — are a huge part of everyday financial life in Kenya. Most of them are still run on notebooks, WhatsApp threads, and memory. That works, until:

- Someone forgets who paid this cycle
- There's a dispute over whose turn it is to receive the payout
- A member misses a meeting and loses track of their contribution history
- The group meets somewhere with no reliable internet — a living room, a rural gathering spot, a church hall — and any app that assumes constant connectivity simply doesn't work there
- Money needs to be withdrawn, and there's no clear, enforced process for more than one person to agree to it

Existing chama apps (Chamasoft, MyChama, ChamaPro, and others) solve parts of this, but they're built assuming constant internet access, and none combine offline-first reliability with enforced multi-person withdrawal approval.

## The Solution

Chama Salama is a Go-based backend (with a growing UI) that treats a chama's contribution and payout record as a single source of truth — one that works whether or not the group has internet at the moment.

Its core design decisions all trace back to real chama behavior:

- **Offline-first**: contributions can be recorded with zero connectivity and sync automatically once a connection returns, instead of failing or requiring internet to function at all
- **M-Pesa integration (Daraja sandbox)**: online contributions can be paid directly via STK push, with Safaricom's confirmation automatically updating the record
- **Multi-signature withdrawals**: no single person — not even the treasurer — can move money out alone. A withdrawal only becomes approved once at least 3 members have individually signed off on it
- **Transparent roles**: every member is also just a member — admins see the same contribution history everyone else does, plus additional management controls, not a separate hidden view

## How It Works

1. **A member registers** — name, phone or email, password (hashed with bcrypt, never stored in plain text), and must accept terms and conditions (versioned and timestamped) before an account is created. Duplicate phone/email registrations are blocked.
2. **They log in** using either their phone or email plus their password, verified against the stored hash. Invalid credentials return the same generic error whether the account exists or not, so no information leaks about who is or isn't registered.
3. **They complete their profile** — national ID, location, next of kin — details a chama constitution typically requires, added after initial registration rather than as a barrier to signing up.
4. **Contributions get recorded** either directly (already confirmed) or offline (queued as `pending`, with a `sync_queue` entry). Once connectivity returns, a sync process picks up every pending contribution and marks it `synced`.
5. **Real M-Pesa payments** go through Safaricom's Daraja API: an STK push prompt is sent to the payer's phone, and Safaricom's callback (received by the app's own `/callback` endpoint) confirms the result.
6. **The dashboard** pulls all of this together — group balance, the group's contribution amount and frequency, and a live feed of recent contributions.
7. **Withdrawals** are requested by any member, then require 3 separate members to individually approve before the status changes to `approved`. A member can't approve the same withdrawal twice.
8. **Admins** can set the group's contribution amount/frequency and view the full member list — same account, same person, just with additional controls layered on top of their normal member view.

## What's Built So Far

- Member registration, validated, duplicate-checked, T&Cs-enforced
- Login with bcrypt password verification
- Profile completion (national ID, location, next of kin)
- Contribution recording — online and offline, with a working sync queue
- Daraja sandbox integration — access tokens, STK push, and a live, internet-reachable callback endpoint (tested end-to-end via ngrok)
- Admin controls — group settings, member listing
- A styled dashboard showing balance, settings, and recent contributions
- Multi-signature withdrawal approval, with duplicate-approval protection
- A deployed, branded static landing page

## What's Not Built Yet (Stretch)

- Group chat / issue raising
- Fines and penalties for late contributions
- Voting/polls for group decisions
- SMS notifications
- True cryptographic digital signatures for withdrawal approval (currently a simplified confirmation, not public/private key signing)
- USSD access for members without smartphones
- A publicly deployed backend (currently runs locally; only the static landing page is live)

## Tech Stack

- **Backend:** Go, using the standard `net/http` package
- **Database:** SQLite, via the pure-Go `modernc.org/sqlite` driver (no C compiler required)
- **Password hashing:** bcrypt
- **Payments:** Safaricom Daraja API (sandbox only — no real money is processed)
- **Version control:** Git, hosted on GitHub, with a feature-branch workflow throughout
- **Landing page:** static HTML/CSS, deployed via GitHub Pages

## Running Locally

```bash
git clone https://github.com/Nelvine201/chama-salama.git
cd chama-salama

# Build the database from schema
sqlite3 chama.db < schema.sql

# Set Daraja sandbox credentials (optional, only needed for payment testing)
export DARAJA_CONSUMER_KEY="your_key"
export DARAJA_CONSUMER_SECRET="your_secret"

go run .
```

Server starts on `http://localhost:8080`.

## Project Status

This is a personal training project for LakeHub Zone01 (Q1). It is not a production financial system — no real money is processed, and Daraja integration runs in sandbox mode only.
