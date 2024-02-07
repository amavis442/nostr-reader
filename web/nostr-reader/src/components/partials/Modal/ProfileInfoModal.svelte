<script lang="ts">
  import { closeModal } from "svelte-modals";
  import Button from "../Button.svelte";

  // provided by <Modals />
  export let isOpen: boolean;
  export let profile;
</script>

{#if isOpen}
  <div role="dialog" class="modal">
    <div class="contents w-1/2">
      <form>
        <h5 class="text-gray-900 text-xl font-medium mb-2">
          Metadata for profile: {profile.name}
        </h5>
        <div class="flex flex-col p-2 w-full">
          <p><span style="font-weight:bold">About:</span> {profile.about}</p>
          <p><span style="font-weight:bold">Website:</span> {profile.website}</p>
         </div>
        <div class="flex space-x-1 p-2">
          <div class="w-6/12 flex justify-end">
            <Button click={closeModal} class="bg-red-500 hover:bg-red-700"
              >Close</Button
            >
          </div>
        </div>
      </form>
    </div>
  </div>
{/if}

<style lang="postcss">
  .modal {
    position: fixed;
    top: 0;
    bottom: 0;
    right: 0;
    left: 0;
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 100;

    /* allow click-through to backdrop */
    pointer-events: none;
  }

  .contents {
    min-width: 460px;
    border-radius: 6px;
    padding: 16px;
    background: white;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    pointer-events: auto;
  }
</style>
