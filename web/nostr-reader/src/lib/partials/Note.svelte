<script>
// @ts-nocheck

    import { createEventDispatcher } from 'svelte';
    import { toHtml, findLink } from "../util/html";
    import Preview from "./Preview/Preview.svelte";
    import placeholder from '../../assets/profile-picture.jpg';

    const dispatch = createEventDispatcher();
    export let note;

    function searchEvent(id, etag) {
        dispatch('searchEvent',{id:id, etag:etag})
    }
    
    function followUser(pubkey) {
        dispatch('followUser', pubkey)
    }
    
    function blockUser(pubkey) {
        dispatch('blockUser', pubkey)
    }
    
    function reply(note) {
        dispatch('reply', note)
    }
</script>
<div class="rounded-t-lg overflow-y-auto border-t border-l border-r border-gray-400 p-10 flex justify-center">
    <div class="max-w-sm w-full lg:max-w-full lg:flex">
      <div class="h-48 lg:h-auto lg:w-48 flex-none bg-cover rounded-t lg:rounded-t-none lg:rounded-l text-center overflow-hidden" style="background-image: url('https://images.alphacoders.com/188/188121.jpg')" title="Mountain">
      </div>
      <div class="border-r border-b border-l border-gray-400 lg:border-l-0 lg:border-t lg:border-gray-400 bg-white rounded-b lg:rounded-b-none lg:rounded-r p-4 flex flex-col justify-between leading-normal">
        <div class="mb-8 text-left text-clip">
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
              <p><small><button on:click="{searchEvent(note.id, etag)}">{etag}</button></small></p>
              <div id="search_{note.id}_{etag}" class="border border-gray-400 p-10 overflow-y-auto rounded"></div>
            {/each}
          {/if}
        </div>
        <div class="flex items-center">
          <img class="w-10 h-10 rounded-full mr-4" src="{ note.profile.picture ? note.profile.picture : placeholder}" alt="Placeholder" title="{ note.profile.about ? note.profile.about : ''}">
          <div class="text-sm text-left">
            <p class="text-gray-900 leading-none">{ note.profile.display_name ? note.profile.display_name : note.profile.name.slice(0, 20) } 
              <button on:click="{followUser(note.pubkey)}">Follow</button>| 
              <button on:click="{blockUser(note.pubkey)}">Block</button>| 
              <button on:click="{reply(note)}">Reply</button></p>
            <p class="text-gray-600"><small>{ (new Date(note.created_at  * 1000)).toLocaleString('nl-NL') }</small></p>
          </div>
        </div>
      </div>
    </div>
  </div>

