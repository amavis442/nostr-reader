<script lang="ts">
   import { createEventDispatcher} from 'svelte';
  import { closeModal } from "svelte-modals";
  import Button from "../Button.svelte";
  import emojiData from './data/emoji.js';
  import { Tabs, Tab, TabList, TabPanel } from '../Tabs/index.js';
  
  import EmojiDetail from './EmojiDetail.svelte';
  import EmojiList from './EmojiList.svelte';
  import EmojiSearch from './EmojiSearch.svelte';
  import EmojiSearchResults from './EmojiSearchResults.svelte';
  import VariantPopup from './VariantPopup.svelte';
  
  export let onAddEmoji: Function;

  export let maxRecents = 50;
  export let autoClose = true;

  export let isOpen: boolean;
  
  let searchText:string = '';
  let variantsVisible = false;
    
  let variants: any;
  let currentEmoji: any;
  let recent = localStorage.getItem('svelte-emoji-picker-recent')
  let recentEmojis = recent != null ? JSON.parse(recent) : [];

  const dispatch = createEventDispatcher();

  const emojiCategories = {};
    emojiData.forEach(emoji => {
      let categoryList = emojiCategories[emoji.category];
      if (!categoryList) {
        categoryList = emojiCategories[emoji.category] = [];
      }
  
      categoryList.push(emoji);
    });
  
    const categoryOrder = [
      'Smileys & People',
      'Animals & Nature',
      'Food & Drink',
      'Activities',
      'Travel & Places',
      'Objects',
      'Symbols',
      'Flags'
    ];
  
    const categoryIcons = {
      'Smileys & People': '😀',
      'Animals & Nature': '😸',
      'Food & Drink': '☕',
      'Activities': '⚽',
      'Travel & Places': '🏡',
      'Objects': '💡',
      'Symbols': '🎵',
      'Flags': '🚩'
    };

    function showEmojiDetails(event) {
      currentEmoji = event.detail;
    }
  
    function onEmojiClick(event) {
      if (event.detail.variants) {
        variants = event.detail.variants;
        variantsVisible = true;
      } else {
        dispatch('emoji', event.detail.emoji);
        saveRecent(event.detail);
        onAddEmoji(event.detail.emoji);

        if (autoClose) {
          closeModal();
        }
      }
    }
  
    function onVariantClick(event) {
      onAddEmoji(event.detail.emoji);
      //dispatch('emoji', event.detail.emoji);
      saveRecent(event.detail);
      hideVariants();
  
      if (autoClose) {
        closeModal();
      }
    }
  
    function saveRecent(emoji) {
      recentEmojis = [emoji, ...recentEmojis.filter(recent => recent.key !== emoji.key)].slice(0, maxRecents);
      localStorage.setItem('svelte-emoji-picker-recent', JSON.stringify(recentEmojis));
    }
  
    function hideVariants() {
      // We have to defer the removal of the variants popup.
      // Otherwise, it gets removed before the click event on the body
      // happens, and the target will have a `null` parent, which
      // means it will not be excluded and the clickoutside event will fire.
      setTimeout(() => {
        variantsVisible = false;
      });
    }

    function doNothing() {}
</script>

{#if isOpen}
  <div role="dialog" class="modal">
    <div class="contents w-1/2">
      <form>
        <h5 class="text-gray-900 text-xl font-medium mb-2">
          Le emojis
        </h5>
        <div class="flex flex-col p-2 w-full">
          <EmojiSearch bind:searchText={searchText} />
          {#if searchText}
          <EmojiSearchResults searchText={searchText} on:emojihover={showEmojiDetails} on:emojiclick={onEmojiClick} />
        {:else}
          <div class="svelte-emoji-picker__emoji-tabs">
            <Tabs selectedTabIndex={1}> 
              <TabList>
                <Tab on:keyup={doNothing}>🕐</Tab>
                {#each categoryOrder as category}
                  <Tab>{categoryIcons[category]}</Tab>
                {/each}
              </TabList>
  
              <TabPanel>
                <EmojiList name="Recently Used" emojis={recentEmojis} on:emojihover={showEmojiDetails} on:emojiclick={onEmojiClick} />
              </TabPanel>
  
              {#each categoryOrder as category}
                <TabPanel>
                  <EmojiList name={category} emojis={emojiCategories[category]} on:emojihover={showEmojiDetails} on:emojiclick={onEmojiClick} />
                </TabPanel>
              {/each}
            </Tabs>
          </div>
        {/if}
  
        {#if variantsVisible}
          <VariantPopup variants={variants} on:emojiclick={onVariantClick} on:close={hideVariants} />
        {/if}
  
        <EmojiDetail emoji={currentEmoji} />



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

<style>
  .modal {
    position: fixed;
    top: 0;
    bottom: 0;
    right: 0;
    left: 0;
    display: flex;
    justify-content: center;
    align-items: center;

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
