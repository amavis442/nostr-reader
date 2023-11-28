# Intro

**This WIP uses svelte as GUI and golang and postgresql as backend.**

In config.json (rename config.json.dist to config.json) you tell which relays you want to read from and in the gui you press the sync button to get the latest events and user metadata and then refreshView or just reload the browser.
I do not concern myself with completeness. You can not even respond yet or give likes. If an event is a response and if the data is in the database,
it will show that after clicking the id. If not, you are out of luck.

There is a block function based on blocking pubkeys, but that is as fancy as it gets.

This is a project to learn golang better.

Inspiration is taken from [https://github.com/mattn/algia](algia)


## Use 

You will need go 1.20 and npm 9.1

`go build .`

this will get the packages and build relaystore.exe on windows and will be the server 

```
cd web/nostr-reader

npm install

npm run build
```

start relaystore.exe, make sure you have a valid config.json with at least your pubkey and private key

In browser goto http://localhost:8080/ and press sync to get events
