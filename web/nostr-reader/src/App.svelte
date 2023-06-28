<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from "svelte";
  import { get, writable, type Writable } from "svelte/store";
  import placeholder from './assets/profile-picture.jpg';
  import { toHtml, findLink } from "./lib/util/html";
  import Preview from "./lib/partials/Preview/Preview.svelte";
  import Pagination from './lib/partials/Pagination.svelte';

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

  onMount(async () => {
    await refreshView({page:1,limit:limit});
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
    fetch("/api/getnext")
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
        refreshView({page:current_page});
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
  
  {#if total > per_page}
  <Pagination
    {current_page}
    {last_page}
    {per_page}
    {from}
    {to}
    {total}
    on:change="{(ev) => refreshView({page: ev.detail, limit: limit})}">
  </Pagination>
{/if}

  <div class="p-10 mb-10">  
    {#each pageData ? pageData : [] as note (note.id)}
    <div class="rounded-t-lg overflow-hidden border-t border-l border-r border-gray-400 p-10 flex justify-center">
      <div class="max-w-sm w-full lg:max-w-full lg:flex">
        <div class="h-48 lg:h-auto lg:w-48 flex-none bg-cover rounded-t lg:rounded-t-none lg:rounded-l text-center overflow-hidden" style="background-image: url('https://images.alphacoders.com/188/188121.jpg')" title="Mountain">
        </div>
        <div class="border-r border-b border-l border-gray-400 lg:border-l-0 lg:border-t lg:border-gray-400 bg-white rounded-b lg:rounded-b-none lg:rounded-r p-4 flex flex-col justify-between leading-normal">
          <div class="mb-8 text-left">
            <small>Event id: {note.id}</small>
            <hr/>
            <p class="text-gray-700 text-base">
              {@html toHtml(note.content)}
              {#if findLink(note.content)}
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <div class="mt-2" on:click={(e) => e.stopPropagation()}>
                <Preview
                  endpoint={`${
                    import.meta.env.VITE_PREVIEW_LINK
                  }/api/preview/link`}
                  url={findLink(note.content)}
                />
              </div>
            {/if}
            </p>
            {#if note.etags && note.etags.length > 0}
              Response to:
              {#each note.etags as etag}
                <p><small><button on:click="{searchEvents(note.id, etag)}">{etag}</button></small></p>
                <div id="search_{note.id}_{etag}" class="border-t border-l border-r border-gray-400 p-10"></div>
              {/each}
            {/if}
          </div>
          <div class="flex items-center">
            <img class="w-10 h-10 rounded-full mr-4" src="{ note.profile.picture ? note.profile.picture : placeholder}" alt="Placeholder" title="{ note.profile.about ? note.profile.about : ''}">
            <div class="text-sm text-left">
              <p class="text-gray-900 leading-none">{ note.profile.display_name ? note.profile.display_name : note.profile.name.slice(0, 20) } <button on:click="{followUser(note.pubkey)}">Follow</button>| <button on:click="{blockUser(note.pubkey)}">Block</button></p>
              <p class="text-gray-600"><small>{ (new Date(note.created_at  * 1000)).toLocaleString('nl-NL') }</small></p>
            </div>
          </div>
        </div>
      </div>
    </div>
    {/each}
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
