<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from "svelte";
  import { get, writable, type Writable } from "svelte/store";
  import placeholder from './assets/profile-picture.jpg';
  import { toHtml, findLink } from "./lib/util/html";
  import Preview from "./lib/partials/Preview/Preview.svelte";
  import Pagination from './lib/partials/Pagination.svelte';

  import NoteEvent from './lib/partials/Note.svelte';
  

  let page = writable([]);
  let pageData = []

  let current_page = 1;
  let from = 1;
  let to = 1;
  let per_page = 1;
  let last_page = 1;
  let total = 0;
  let loading = true;
  let limit = 30
  let since = 1 // 1 day

  onMount(async () => {
    await refreshView({page:1,limit:limit, since:since});
  });

async function refreshView(params) {
  await fetch("/api/events", {
      method: "POST",
      body: JSON.stringify(params),
      headers: {
        "Content-Type": "application/json",
      },
    })
      .then((res) => {
        return res.json();
      })
      .then((data) => {
        console.log("Json is ", data);


        current_page = data.current_page;
        from = data.from;
        to = data.to;
        per_page = data.per_page;
        last_page = data.last_page;
        total = data.total;

        pageData = data.data
      })
      .catch((err) => {
        console.error("error", err);
      });
}


  async function refresh() {
    fetch("/api/sync")
  }

  function blockUser(pubkey) {
    fetch("/api/blockuser", {
      method: "POST",
      body: JSON.stringify({ pubkey: pubkey }),
      headers: {
        "Content-Type": "application/json",
      }
    }).then((res) => {
        return res.json();
      })
      .then((data) => {
        console.log("Json is ", data);
        refreshView({page:current_page, limit: limit, since:since});
        return data;
      })
      .catch((err) => {
        console.error("error", err);
      });
  }
  
  function followUser(pubkey) {
    fetch("/api/followuser", {
      method: "POST",
      body: JSON.stringify({ pubkey: pubkey }),
      headers: {
        "Content-Type": "application/json",
      }
    }).then((res) => {
        return res.json();
      })
      .then((data) => {
        console.log("Json is ", data);
        return data;
      })
      .catch((err) => {
        console.error("error", err);
      });
  }

  let searchEvent = writable({})
  function searchEvents(noteId, etag) 
  {
    fetch("/api/searchevent", {
      method: "POST",
      body: JSON.stringify({ id: etag }),
      headers: {
        "Content-Type": "application/json",
      }
    }).then((res) => {
        return res.json();
      })
      .then((data) => {
        console.log("Json is ", data);
        if (data.content) {
          document.getElementById('search_' + noteId + '_' + etag).innerHTML = toHtml(data.content) + '<br/>' + data.profile.name;
        }
        if (!data.content) {
          document.getElementById('search_' + noteId + '_' + etag).innerHTML = "No event data available";
        }
        
        return data;
      })
      .catch((err) => {
        console.error("error", err);
      });
  }
</script>

<main>
  <button on:click="{refresh}" class="btn btn-blue">Sync</button>
  <select id="limit" bind:value={limit} on:change={() => (refreshView({page:current_page,limit:limit, since:since }))} class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500">
		{#each [10,15,20,25,30] as limitValue}
			<option value={limitValue}>
				{limitValue}
			</option>
		{/each}
	</select><label for="limit">Items Per Page</label>
  
  <select id="since" bind:value={since} on:change={() => (refreshView({page:current_page,limit:limit, since:since }))} class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500">
		{#each [1,2,3,4,5,6,7] as sinceValue}
			<option value={sinceValue}>
				{sinceValue}
			</option>
		{/each}
	</select> <label for="since"> Days (since)</label>

  {#if total > per_page}
    <Pagination
      {current_page}
      {last_page}
      {per_page}
      {from}
      {to}
      {total}
      on:change="{(ev) => refreshView({page: ev.detail, limit: limit, since: since})}">
    </Pagination>
  {/if}

  <div class="p-10 mb-10">  
    <div class="flex flex-col gap-4 h-screen">
      <div class="h-screen">
        <div
          id="Notes"
          class="flex flex-col relative mx-auto bg-gray-800
                dark:highlight-white/5 shadow-lg ring-1 ring-black/5
                divide-y ml-4 mr-4
                space-y-0 place-content-start
                h-full max-h-full w-11/12"
        >
            <div class="h-full w-full overflow-y-auto">
      {#each pageData ? pageData : [] as note (note.id)}
        <NoteEvent {note}
        on:searchEvent="{(ev) => searchEvents(ev.detail.id, ev.detail.etag)}"
        on:followUser="{(ev) => followUser(ev.detail)}"
        on:blockUser="{(ev) => blockUser(ev.detail)}"
        ></NoteEvent>
      {/each}
    </div>
  </div>
</div>
</div>
</main>


<style>
  .btn {
    @apply font-bold py-2 px-4 rounded;
  }
  .btn-blue {
    @apply bg-blue-500 text-white;
  }
  .btn-blue:hover {
    @apply bg-blue-700;
  }
</style>
