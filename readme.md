# Intro

**This WIP uses svelte as GUI and golang and postgresql as backend.**

In config.json (rename config.json.dist to config.json) you tell which relays you want to read from and in the gui you press the sync button to get the latest events and user metadata and then refreshView or just reload the browser.
I do not concern myself with completeness. You can not even respond yet or give likes. If an event is a response and if the data is in the database,
it will show that after clicking the id. If not, you are out of luck.

There is a block function based on blocking pubkeys, but that is as fancy as it gets.

This is a project to learn golang better.
