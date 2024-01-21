# Intro

**This WIP uses svelte as GUI and golang and postgresql as backend.**

In config.json (rename config.json.dist to config.json) you tell which relays you want to read from and 
in the gui you press the sync button to get the latest events and user metadata the view will be refreshed automatically.
A sample of the content of notr relays is porbed every 5 minutes. Only you own replies wil be shown realtime.

I do not concern myself with completeness. If an event is a response and if the data is in the database,
it will show that after clicking the show/hide button. If not, you are out of luck.

There is a block function based on blocking pubkeys, but that is as fancy as it gets.

This is a project to learn the go language better.

Inspiration is taken from [algia](https://github.com/mattn/algia) and [gnost-relay](https://github.com/barkyq/gnost-relay)
And biggest thanks goes to fiatjaf [go-nostr](https://github.com/nbd-wtf/go-nostr) for making life easier.

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

## Use 

You will need go 1.20 and npm 9.1

`go build .`

This will get the packages and build relaystore.exe on windows and will be the server. 

For the frontend rename .env.local.dist to .env.local.


```
cd web/nostr-reader

npm install

npm run build
```

This will get the packages and build the frontend. it will be stored in the dist folder. 
For the build of the web gui you can set some parameters in .env.local. If you want to use translate then
install [libretranslate](https://github.com/argosopentech/LibreTranslate-init.git). This is a python app 
and it downloads some big translate models. Leave VITE_APP_TRANSLATE_URL empty to disable the translate function.
When you make changes to **.env.local** then you need to rebuild the web frontend with ```npm run build``` 


Start relaystore.exe, make sure you have a valid config.json with at least your pubkey and private key.
Also start the postgresql server on the standard port or change that in the config file. If something goes run, then run the app in a console 
for debugging info.

In browser goto http://localhost:8080/ and press sync to get events. If you want to set another port then change that in config.json and in .env.local and rebuild the server and the frontend.


## License

MIT
