<script>
    import { onMount } from 'svelte';
  
    export let searchText = '';
  
    /**
	 * @type {HTMLInputElement}
	 */
    let searchField;
  
    onMount(() => {
      searchField.focus();
    });
  
    function clearSearchText() {
      searchText = '';
      searchField.focus();
    }
  
    /**
	 * @param {{ key: string; stopPropagation: () => void; }} event
	 */
    function handleKeyDown(event) {
      if (event.key === 'Escape' && searchText) {
        clearSearchText();
        event.stopPropagation();
      }
    }

    function doNothing(){}
  </script>
  
  <style>
    .svelte-emoji-picker__search {
      padding: 0.25em;
      position: relative;
    }
  
    .svelte-emoji-picker__search input {
      width: 100%;
      border-radius: 5px;
    }
  
    .svelte-emoji-picker__search input:focus {
      outline: none;
      border-color: #4F81E5;
    }
  
    .icon {
      color: #AAAAAA;
      position: absolute;
      font-size: 1em;
      top: calc(50% - 0.5em);
      right: 0.75em;
    }
  
    .icon.clear-button {
      cursor: pointer;
    }
  </style>
  
  <div class="svelte-emoji-picker__search">
    <input type="text" placeholder="Search emojis..." bind:value={searchText} bind:this={searchField} on:keydown={handleKeyDown}>
    
    {#if searchText}
      <span class="icon clear-button" role="button" on:click|stopPropagation={clearSearchText} tabindex="0" on:keyup={doNothing}>🕐</span>
    {:else}
      <span class="icon">🔍</span>
    {/if}
  </div>