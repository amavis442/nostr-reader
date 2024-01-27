<script lang="ts">
  // @ts-nocheck

  import { createEventDispatcher, beforeUpdate } from "svelte";
  import { toHtml, findLink } from "./util/html";
  import Preview from "./partials/Preview/Preview.svelte";
  import placeholder from "../assets/profile-picture.jpg";
  import Icon from "svelte-icons-pack/Icon.svelte";
  import FaSolidInfoCircle from "svelte-icons-pack/fa/FaSolidInfoCircle";
  import FaSolidUserMinus from "svelte-icons-pack/fa/FaSolidUserMinus";
  import FaSolidUserPlus from "svelte-icons-pack/fa/FaSolidUserPlus";

  import FaBookmark from "svelte-icons-pack/fa/FaBookmark";
  import FaSolidBookmark from "svelte-icons-pack/fa/FaSolidBookmark";

  import FaFolder from "svelte-icons-pack/fa/FaFolder";
  import FaFolderOpen from "svelte-icons-pack/fa/FaFolderOpen";

  
  import FaSolidBan from "svelte-icons-pack/fa/FaSolidBan";
  import FaCommentDots from "svelte-icons-pack/fa/FaCommentDots";
  import FaSolidSync from "svelte-icons-pack/fa/FaSolidSync";
  import FaSolidLongArrowAltUp from "svelte-icons-pack/fa/FaSolidLongArrowAltUp";
  import { tranlateContent } from "./state/main";

  const dispatch = createEventDispatcher();
  export let note;

  function followUser(pubkey) {
    dispatch("followUser", pubkey);
  }

  function unfollowUser(pubkey) {
    dispatch("unfollowUser", pubkey);
  }

  function addBookmark(eventID) {
    dispatch("addBookmark", eventID);
  }

  function removeBookmark(eventID) {
    dispatch("removeBookmark", eventID);
  }

  function blockUser(pubkey) {
    if (confirm("Block user?") == true) {
      dispatch("blockUser", pubkey);
    }
  }

  function reply(note) {
    dispatch("reply", note);
  }

  function info(note) {
    dispatch("info", note);
  }
  function syncnote(note) {
    dispatch("syncNote", note);
  }

  function gotoTopOfPage(note) {
    dispatch("topPage", note);
  }

  let repliesExpanded = false;
  function toggleReplies() {
    repliesExpanded = !repliesExpanded;
  }

  function normalizeName(data): string {
    return (
      data ? (data.name ? data.name : note.event.pubkey) : note.event.pubkey
    ).slice(0, data && data.name.length < 50 ? data.name.length : 20);
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

  let imgUrls = [];
  let hasImgUrls = false;
  let content = "";

  beforeUpdate(() => {
    imgUrls = findLink(note.event.content);

    if (imgUrls && imgUrls.length > 0) {
      console.log("Img/youtube/rumble urls\n", imgUrls);
      hasImgUrls = true;
    }

    content = toHtml(note.event.content);
  });

  let translatedContent = "";
  async function tranlate() {
    translatedContent = await tranlateContent(note.event.content);
  }
</script>

{#if note && note.event.kind == 1}
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
              alt={note.event.pubkey.slice(0, 10)}
            />
          </div>

          <div class="flex-col w-full">
            <div class="px-2">
              <div class="h-12">
                <div class="flex gap-2 h-12 w-full">
                  <div class="text-left order-first w-6/12">
                    <strong class="text-black text-sm font-medium">
                      <span title={note.event.pubkey}
                        >{normalizeName(note.profile)}</span
                      >
                      {#if note.profile.followed}
                        <i class="fa-solid fa-bookmark" />
                      {/if}
                      <small class="text-gray"
                        >{new Date(note.event.created_at * 1000).toLocaleString(
                          "nl-NL"
                        )}</small
                      >
                    </strong>
                  </div>

                  <div class="text-right order-last md:w-6/12">
                    <span class="text-right">
                      <div
                        class="flex flex-row gap-1 content-normal justify-end"
                      >
                        <div>
                          {#if note.profile.followed}
                            <button
                              on:click={unfollowUser(note.event.pubkey)}
                              title="unfollow"
                              class="p-1"
                              ><Icon src={FaSolidUserMinus} size="24" /></button
                            >
                          {:else}
                            <button
                              on:click={followUser(note.event.pubkey)}
                              title="follow"
                              class="p-1"
                              ><Icon src={FaSolidUserPlus} size="24" /></button
                            >
                          {/if}
                        </div>
                        
                        <div>
                          {#if note.bookmark}
                            <button
                              on:click={removeBookmark(note.event.id)}
                              title="remove bookmark"
                              class="p-1"
                              ><Icon src={FaSolidBookmark} size="24" /></button
                            >
                          {:else}
                            <button
                              on:click={addBookmark(note.event.id)}
                              title="add bookmark"
                              class="p-1"
                              ><Icon src={FaBookmark} size="24" /></button
                            >
                          {/if}
                        </div>
                        
                        <div>
                          <button
                            on:click={reply(note)}
                            title="reply"
                            class="p-1"
                            ><Icon src={FaCommentDots} size="24" /></button
                          >
                        </div>
                        <div>
                          <button on:click={info(note)} title="info" class="p-1"
                            ><Icon src={FaSolidInfoCircle} size="24" /></button
                          >
                        </div>
                        <div>
                          <button
                            on:click={syncnote(note)}
                            title="sync note"
                            class="p-1"
                            ><Icon src={FaSolidSync} size="24" /></button
                          >
                        </div>
                        <div>
                          <button
                            on:click={blockUser(note.event.pubkey)}
                            class="p-1"
                            title="block"
                            ><Icon src={FaSolidBan} size="24" /></button
                          >
                        </div>
                        <div>
                          <button
                            on:click={gotoTopOfPage(note)}
                            class="p-1"
                            title="block"
                            ><Icon
                              src={FaSolidLongArrowAltUp}
                              size="24"
                            /></button
                          >
                        </div>
                      </div>
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="p-2 w-11/12">
              <div class="text-left w-full max-w-max break-words items-top">
                <span class="text-black text-md font-medium break-words">
                  {@html content}
                  {#if import.meta.env.VITE_APP_TRANSLATE_URL && import.meta.env.VITE_APP_TRANSLATE_LANG}
                  <button
                    on:click={tranlate}
                    class="p-1 m-2"
                    title="Translate">Translate to ({import.meta.env.VITE_APP_TRANSLATE_LANG})</button
                  >
                    {#if translatedContent != ""}
                      <div
                        id="translateContent_{note.event.id}"
                        class="rounded-2xl border border-solid border-medium bg-white p-4 mt-2 mb-2"
                      >
                        {translatedContent}
                      </div>
                    {/if}
                  {/if}
                </span>
                {#if hasImgUrls}
                  <!-- svelte-ignore a11y-click-events-have-key-events -->
                  {#each imgUrls as s, outerIndex}
                    {#if outerIndex % 3 === 0}
                      <div
                        class="mt-4 flex flex-cols-2 gap-4 bg-bg_color"
                        on:click={(e) => e.stopPropagation()}
                      >
                        {#each imgUrls as imgUrl, i}
                          {#if i >= outerIndex && i < outerIndex + 3}
                            <Preview
                              endpoint={`${
                                import.meta.env.VITE_API_LINK
                              }/api/preview/link`}
                              url={imgUrl}
                            />
                          {/if}
                        {/each}
                      </div>
                    {/if}
                  {/each}
                {/if}
              </div>
            </div>

            <div class="w-full">
              <p class="mt-4 flex space-x-8 w-full p-1">
                <span>
                  {#if note.children && Object.keys(note.children).length > 0}
                    <button type="button" on:click={toggleReplies} class="">
                      {#if repliesExpanded}
                        <Icon src={FaFolderOpen} size="24" className="inline"/> 
                      {:else}
                        <Icon src={FaFolder} size="24" className="inline"/> 
                      {/if}
                      <small>({Object.keys(note.children).length})</small>
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
                    on:info
                    on:topPage
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
