# Show HN Draft

## Title

Show HN: WhatSignal – A self-hosted WhatsApp-to-Signal bridge in Go

## Creator's Comment

I built this because I got tired of switching between two messaging apps. Most of my contacts are on WhatsApp but I'd rather use Signal, so I wrote a bridge. WhatsApp messages show up in Signal, I reply from Signal, the other person doesn't know the difference.

Three Docker containers: WAHA (WhatsApp Web API), signal-cli-rest-api (Signal protocol client), and the bridge (Go binary that does the routing).

The problem I burned the most time on was reply routing. When you quote a message on Signal and reply, the bridge has to figure out which WhatsApp contact that reply belongs to. Signal uses timestamps as message IDs. WhatsApp uses opaque `wamid.*` strings. And signal-cli, sitting in the middle, changed its JSON serialization between versions — at one point it was putting the quote field under three different names depending on the operating mode. My changelog is honestly just a debugging diary of chasing these.

On the storage side — all sensitive fields in the DB (chat IDs, phone numbers, etc.) are AES-256-GCM encrypted with random nonces. Problem is you can't do WHERE clauses on randomly-nonced ciphertext, so each field also gets a companion HMAC-SHA256 hash column for lookups. Lets me avoid deterministic encryption without giving up query performance.

signal-cli has two modes: `native` (HTTP polling) and `json-rpc` (WebSocket). Native silently strips quote metadata from messages, which was a fun one to debug. json-rpc keeps it but has historically been unreliable. The bridge detects which mode is running at startup and adjusts.

The Signal receive endpoint is also destructive — dequeues messages on read. If the bridge crashes after receiving but before saving, they're gone. So I write partial mappings before network calls and replay orphaned messages on restart.

Self-hosted, no cloud, MIT licensed, SQLite, runs on a Pi.

I should be upfront about the limitations: it depends on WAHA, which automates WhatsApp Web, so you're at WhatsApp's mercy re: unofficial clients. Ban risk is real. I've run it daily for 9+ months without issues but YMMV.

https://github.com/bikemazzell/whatsignal

Happy to dig into the routing logic, the encryption design, or any of the gnarly edge cases if people are curious.

## Pre-emptive FAQ

### "Does this break end-to-end encryption?"
WAHA decrypts WhatsApp messages on your machine, signal-cli re-encrypts them for Signal. Plaintext exists momentarily on your server, same as when you read a message on your phone. No third party sees anything. The SQLite DB encrypts sensitive fields at rest with AES-256-GCM.

### "What about privacy for the people messaging you?"
Their messages only pass through your own infrastructure. Nothing leaves your server. The bridge stores encrypted routing metadata (message IDs, chat IDs) for reply threading, not message content. Architecturally it's the same as WhatsApp Web on your laptop, except the display goes to Signal instead of a browser tab.

### "Doesn't this violate WhatsApp ToS?"
Technically yes -- any unofficial API usage does. Same goes for Beeper, which operated openly for years. I've run this daily for 9+ months without issues. The risk is real but manageable for personal use.

### "Why not just use Matrix/Beeper?"
Beeper is hosted (now part of Automattic). Matrix bridges work but they're heavy -- you need a full homeserver. WhatSignal is three Docker containers and a SQLite file. No account creation, no federation, no homeserver. It's a point-to-point bridge for one person.

### "Why not just use Signal for everything / convince your contacts to switch?"
Been trying for years. Some people won't move. This is the pragmatic answer.

### "Is this the Nth WhatsApp bridge?"
Most WhatsApp tools out there are API wrappers, MCP servers, or Matrix bridges. This is a bidirectional bridge that makes Signal your single inbox for both protocols, with threading, reactions, and media intact. The routing logic for keeping conversation context across two completely different ID schemes is where the interesting work is.
