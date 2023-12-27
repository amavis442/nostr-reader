# Intro

**This WIP uses svelte as GUI and golang and postgresql as backend.**

In config.json (rename config.json.dist to config.json) you tell which relays you want to read from and in the gui you press the sync button to get the latest events and user metadata ~~and then refreshView or just reload the browser~~ the view will be refreshed automatically.
I do not concern myself with completeness. You can not even ~~respond yet or~~ give likes. If an event is a response and if the data is in the database,
it will show that after clicking the show/hide button. If not, you are out of luck.

There is a block function based on blocking pubkeys, but that is as fancy as it gets.

This is a project to learn the go language better.

Inspiration is taken from [https://github.com/mattn/algia](algia)


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

This will get the packages and build the frontend. it will be storeed in the dist folder. 

Start relaystore.exe, make sure you have a valid config.json with at least your pubkey and private key.
Also start the postgresql server on the standard port or change that in the config file. A log file in 
the project dir will show if something goes wrong and why.
For now there is a lot of logging in the file like sql query's, event data etc. This is for debugging 
purposes and will be optional in the future. 

In browser goto http://localhost:8080/ and press sync to get events. The port is fixed for now and can give problems if you have other
apps running on that same port. If you want to set another port then change that in config.json and in .env.local and rebuild the server and the 
frontend.
