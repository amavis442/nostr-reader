<script lang="ts">
  import { onMount } from "svelte";
  import Pagination from "./partials/Pagination.svelte";
  import Feeder from "./Feeder.svelte";
  import TextNote from "./TextNote.svelte";
  import CreateNoteModal from "./partials/Modal/CreateNoteModal.svelte";
  import InfoModal from "./partials/Modal/InfoModal.svelte";
  import { openModal } from "svelte-modals";
  import {
    refreshView,
    refresh,
    blockUser,
    followUser,
    unfollowUser,
    publish,
    pageData,
    setApiUrl,
    pageMetaData,
    syncNote
  } from "./state/main";

  export let apiUrl: string = "";
  onMount(async () => {
    setApiUrl(apiUrl);
    pageData.set([]);
    await refreshView({
      page: 1,
      limit: $pageMetaData.limit,
      since: $pageMetaData.since,
    });
    document.getElementById("content").scrollTo(0, 0)
  });

  function createReplyTextNote(replyToNote) {
    openModal(CreateNoteModal, {
      note: replyToNote,
      onSendTextNote: (noteText: string) => {
        publish(noteText, replyToNote.id);
      },
    });
  }

  function createInfoModal(note) {
    openModal(InfoModal, {
      note: note,
    });
  }
</script>

<main>
  <Feeder>
    <slot>
      <div class="flex flex-col bg-white p-2 rounded-lg m-2">
        <button on:click={refresh} class="btn btn-blue"
          ><i class="fa-solid fa-arrows-rotate"></i> Sync</button
        >
        <select
          id="limit"
          bind:value={$pageMetaData.limit}
          on:change={() => {
            refreshView({
              page: $pageMetaData.current_page,
              limit: $pageMetaData.limit,
              since: $pageMetaData.since,
            });
          }}
          class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
        >
          {#each [10, 15, 20, 25, 30, 60, 90, 120, 150, 180] as limitValue}
            <option value={limitValue}>
              {limitValue}
            </option>
          {/each}
        </select><label for="limit">Items Per Page</label>

        <select
          id="since"
          bind:value={$pageMetaData.since}
          on:change={() =>
            refreshView({
              page: $pageMetaData.current_page,
              limit: $pageMetaData.limit,
              since: $pageMetaData.since,
            })}
          class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
        >
          {#each [1, 2, 3, 4, 5, 6, 7] as sinceValue}
            <option value={sinceValue}>
              {sinceValue}
            </option>
          {/each}
        </select> <label for="since"> Days (since)</label>
        {#if $pageMetaData.total > $pageMetaData.per_page}
          <Pagination
            on:change={async (ev) => {
              await refreshView({
                page: ev.detail,
                limit: $pageMetaData.limit,
                since: $pageMetaData.since,
              });
            }}
          ></Pagination>
        {/if}
      </div>

      <ul class="items-center w-full border-hidden" id="content">
        {#each $pageData ? $pageData : [] as note (note.event.id)}
          <TextNote
            {note}
            on:followUser={(ev) => followUser(ev.detail)}
            on:unfollowUser={(ev) => unfollowUser(ev.detail)}
            on:blockUser={(ev) => blockUser(ev.detail)}
            on:syncNote={(ev) => syncNote(ev.detail)}
            on:reply={(ev) => {
              createReplyTextNote(ev.detail);
            }}
            on:info={(ev) => {
              createInfoModal(ev.detail);
            }}
          ></TextNote>
        {/each}
      </ul>
    </slot>
  </Feeder>
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
