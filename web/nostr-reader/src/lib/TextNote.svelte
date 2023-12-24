<script lang="ts">
  // @ts-nocheck

  import { createEventDispatcher } from "svelte";
  import { toHtml, findLink } from "./util/html";
  import Preview from "./partials/Preview/Preview.svelte";
  import placeholder from "../assets/profile-picture.jpg";

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

  function normalizeName(data): string {
    return (data ? (data.name ? data.name : note.pubkey) : note.pubkey).slice(
      0,
      10
    );
  }

  let borderColor = "border-indigo-" + (note.tree * 100 + 400);

  function align() {
    if (note.tree == 0) return "";
  }

  function firstBlock() {
    if (note.tree === 0) {
      return "border-l-4 border-t-2 " + borderColor;
    }
    return "";
  }

  function childBlock() {
    if (note.tree > 0) {
      return "border-l-4 border-t-2 " + borderColor;
    }

    return "";
  }
</script>

{#if note && note.kind == 1}
  <li>
    <div class="flex flex-col items-top p-2 w-full overflow-hidden mb-2">
      <div
        class="flex flex-col overflow-y-auto bg-white rounded-lg p-1 {firstBlock()} {$$props[
          'class'
        ]
          ? $$props['class']
          : ''}"
      >
        <div
          id={note.id}
          class="flex flex-row w-full min-h-full {align()} items-top gap-2 mb-2 overflow-y-auto bg-white rounded-lg p-1 {childBlock()}"
        >
          <div on:keyup={() => console.log("keyup")} class="w-16 mr-2">
            <img
              class="w-14 h-14 rounded-full {note.profile.followed
                ? 'border-2 border-green-800'
                : ''}"
              src={note.profile.picture != ""
                ? note.profile.picture
                : placeholder}
              title={note.profile.about ? note.profile.about : ""}
              alt={note.pubkey.slice(0, 5)}
            />
          </div>

          <div class="flex-col w-full">
            <div class="px-2">
              <div class="h-12">
                <div class="flex gap-2 h-12 w-full">
                  <div class="text-left order-first w-6/12">
                    <strong class="text-black text-sm font-medium">
                      <span title={note.pubkey}
                        >{normalizeName(note.profile)}</span
                      >
                      {#if note.profile.followed}
                        <i class="fa-solid fa-bookmark" />
                      {/if}
                      <small class="text-gray"
                        >{new Date(note.created_at * 1000).toLocaleString(
                          "nl-NL"
                        )}</small
                      >
                    </strong>
                  </div>

                  <div class="text-right order-last md:w-6/12">
                    <span class="text-right">
                      {#if note.profile.followed}
                        <button on:click={unfollowUser(note.pubkey)}
                          ><i class="fa-solid fa-user-minus"></i> Unfollow</button
                        >|
                      {:else}
                        <button on:click={followUser(note.pubkey)}
                          ><i class="fa-solid fa-user-plus"></i> Follow</button
                        >|
                      {/if}
                      <button on:click={blockUser(note.pubkey)}
                        ><i class="fa-solid fa-ban" /> Block</button
                      >|
                      <button on:click={reply(note)}
                        ><i class="fa-solid fa-comment-dots"></i> Reply</button
                      >
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="xl:max-w-lg md:max-w-lg sm:max-w-sm p-2">
              <div class="text-left w-full max-w-max break-words items-top">
                <span class="text-black text-md font-medium">
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
                </span>
              </div>
            </div>

            <div class="w-full">
              <p class="mt-4 flex space-x-8 w-full p-1">
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
            </div>
          </div>
        </div>
        {#if repliesExpanded}
          {#if note.children && Object.keys(note.children).length > 0}
            <ul>
              {#each Object.values(note.children) as note (note.id)}
                <li>
                  <!--on:banUser is required here so that the event is forwarded-->
                  <!--https://dev.to/mohamadharith/workaround-for-bubbling-custom-events-in-svelte-3khk-->
                  <svelte:self
                    {note}
                    on:followUser
                    on:unfollowUser
                    on:blockUser
                    on:reply
                  />
                </li>
              {/each}
            </ul>
          {/if}
        {/if}
      </div>
    </div>
  </li>
{/if}

<style>
  .border-indigo-100 {
    border-color: rgb(224 231 255);
  }
  .border-indigo-200 {
    border-color: rgb(199 210 254);
  }
  .border-indigo-300 {
    border-color: rgb(165 180 252);
  }
  .border-indigo-400 {
    border-color: rgb(129 140 248);
  }
  .border-indigo-500 {
    border-color: rgb(99 102 241);
  }
  .border-indigo-600 {
    border-color: rgb(79 70 229);
  }
  .border-indigo-700 {
    border-color: rgb(67 56 202);
  }
  .border-indigo-800 {
    border-color: rgb(55 48 163);
  }
  .border-indigo-900 {
    border-color: rgb(49 46 129);
  }

  button {
    @apply p-1 bg-slate-400 rounded ml-1 mr-1 text-white;
  }
</style>
