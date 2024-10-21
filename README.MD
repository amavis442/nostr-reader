# Intro

**This WIP uses golang and postgresql with gorm as ORM.**

This is a split form my previous package where frontend and backend lived together. I split this package (nostr-reader) up in 2.

### Inspiration

Inspiration is taken from [algia](https://github.com/mattn/algia) and [gnost-relay](https://github.com/barkyq/gnost-relay).
And biggest thanks goes to fiatjaf [go-nostr](https://github.com/nbd-wtf/go-nostr) for making life easier.

## Goal

My goal is first of all to learn *go* better and to make a kind of twitter/X clone where you can use to read notes in peace, without getting distracted with the stream and flow of data. Maybe later i will add more features like zap and such. ***It is not my goal to make it mobile or running everywhere. It will always be a desktop app running locally, so you stay in control.***

## Requirements

You need [go](https://go.dev/) version 1.20 to build the source. If you want the translate function, you will also need to install [libretranslate](https://github.com/LibreTranslate/LibreTranslate). This is a python app which means you will have to install python as well.

The server uses ***postgresql*** version > 15 as backend. On windows you can either use [wsl2](https://learn.microsoft.com/en-us/windows/wsl/install) or [msys2](https://www.msys2.org/). For linux it is straight forward.

## Installation and build

### Installation
```
git clone https://github.com/amavis442/nostr-reader-server.git 
```
This needs [git](https://git-scm.com/) installed or get the zip from github.

### Build
```
cd nostr-reader-server

go build . // will build nostr-reader

```

This will get the packages and build the frontend. 

## Configuration

### Server 

Minimal configuration. Need to be at ~/.config/nostr-reader/config.json or in [User Homedir]/AppData/Roaming/nostr-reader on windows

```
// <UserDir>/AppData/Roaming/nostr-reader/config.json
{
  "database": {
    "user": "xxxxx",
    "password": "xxxxxx",
    "dbname": "xxxxxx",
    "port": 5432,
    "host": "localhost"
  },
  "server": {
      "port": 8080,
  },
  "nostr": {
    "privatekey": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
    "nip05": "https://nostrcheck.me/"
  }
}

```

Private key can also start with nsec and it will have the form of nsecXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

## Features

- [x] Nip 01
- [x] Account update
- [ ] Generate keys (Private/Public)
- [x] Replies
- [ ] Upvotes/Downvotes
- [x] Preview links
- [x] Follow
- [ ] Channels
- [ ] Notifications

## Status as in [Nips](https://github.com/nostr-protocol/nips)

- [x] NIP-01: Basic protocol flow description
- [ ] NIP-02: Contact List and Petnames
- [ ] NIP-04: Encrypted Direct Message
- [ ] NIP-05: Mapping Nostr keys to DNS-based internet identifiers
- [ ] NIP-06: Basic key derivation from mnemonic seed phrase
- [ ] NIP-08: Handling Mentions (just replacement but no search / autocomplete)
- [x] NIP-09: Event Deletion
- [x] NIP-10: Conventions for clients' use of e and p tags in text events.
- [ ] NIP-11: Relay Information Document
- [ ] NIP-12: Generic Tag Queries
- [ ] NIP-14: Subject tag in text events.
- [x] NIP-15: End of Stored Events Notice
- [ ] NIP-16: Event Treatment
- [ ] NIP-19: bech32-encoded entities
- [x] NIP-25: Reactions
- [ ] NIP-26: Delegated Event Signing
- [ ] NIP-28: Public Chat
- [ ] NIP-35: User Discovery
- [ ] NIP-36: Sensitive Content
- [ ] NIP-40: Expiration Timestamp


## Execute 

Start nostr-reader-server.exe, make sure you have a valid config.json with at least your private key.
Also start the postgresql server on the standard port or change that in the config file. 

The server takes a get data from the relays every 5 minutes. This is to make sure the relays are not getting stressed and you can read the notes in peace without flashing streams.

## License

MIT
