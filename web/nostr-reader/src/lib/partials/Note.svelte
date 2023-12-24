<script>
  // @ts-nocheck

  import { createEventDispatcher } from "svelte";
  import { toHtml, findLink } from "../util/html";
  import Preview from "./Preview/Preview.svelte";
  import placeholder from "../../assets/profile-picture.jpg";

  const dispatch = createEventDispatcher();
  export let note;

  function followUser(pubkey) {
    dispatch("followUser", pubkey);
  }

  function unfollowUser(pubkey) {
    dispatch("unfollowUser", pubkey);
  }

  function blockUser(pubkey) {
    dispatch("blockUser", pubkey);
  }

  function reply(note) {
    dispatch("reply", note);
  }

  let repliesExpanded = false;
  function toggleReplies() {
    repliesExpanded = !repliesExpanded;
  }
</script>

<div
  class="rounded-t-lg overflow-y-auto border-t border-l border-r border-gray-400 p-10 flex justify-center"
  >
  <div class="max-w-sm w-full lg:max-w-full lg:flex">
    <div
      class="border-r border-b border-l border-gray-400 lg:border-l-0 lg:border-t lg:border-gray-400 bg-white rounded-b lg:rounded-b-none lg:rounded-r p-4 flex flex-col justify-between leading-normal"
    >
      <div class="mb-8 text-left text-clip">
        <small>Event id: {note.id}</small>
        <hr />
        <p
          class="text-gray-700 text-base border border-gray-400 p-10 overflow-y-auto rounded"
        >
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
      </div>
      <div class="flex items-center">
        <img
          class="w-10 h-10 rounded-full mr-4 {note.profile.followed
            ? 'border-2 border-green-800'
            : ''}"
          src={note.profile.picture ? note.profile.picture : placeholder}
          alt="Placeholder"
          title={note.profile.about ? note.profile.about : ""}
        />
        <div class="text-sm text-left">
          <p class="text-gray-900 leading-none">
            {note.profile.display_name
              ? note.profile.display_name
              : note.profile.name.slice(0, 20)}
            {#if note.profile.followed}
              <button on:click={unfollowUser(note.pubkey)}>Unfollow</button>|
            {:else}
              <button on:click={followUser(note.pubkey)}>Follow</button>|
            {/if}
            <button on:click={blockUser(note.pubkey)}>Block</button>|
            <button on:click={reply(note)}
              >Reply ({note.children
                ? Object.keys(note.children).length
                : 0})</button
            >
            <span>
              {#if note.children && Object.keys(note.children).length > 0}
                <button type="button" on:click={toggleReplies} class="">
                  {#if repliesExpanded}
                    Hide {Object.keys(note.children).length} repl{#if Object.keys(note.children).length == 1}y{:else}ies{/if}
                  {:else}
                    Show {Object.keys(note.children).length} repl{#if Object.keys(note.children).length == 1}y{:else}ies{/if}
                  {/if}
                </button>
              {/if}
            </span>
          </p>
          <p class="text-gray-600">
            <small
              >{new Date(note.created_at * 1000).toLocaleString("nl-NL")}</small
            >
          </p>
        </div>
      </div>
    </div>
  </div>
</div>
{#if repliesExpanded}
  {#if note.children && Object.keys(note.children).length > 0}
    <ul class="border-2 border-spacing-2 border-red-700">
      {#each Object.values(note.children) as note (note.id)}
        <li>
          <!--on:banUser is required here so that the event is forwarded-->
          <!--https://dev.to/mohamadharith/workaround-for-bubbling-custom-events-in-svelte-3khk-->
          <svelte:self {note} 
            on:followUser 
            on:unfollowUser
            on:blockUser
            on:reply/>
        </li>
      {/each}
    </ul>
  {/if}
{/if}
