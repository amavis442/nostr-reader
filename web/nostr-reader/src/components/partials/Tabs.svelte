<script>
    import { onMount } from "svelte";
    import { createEventDispatcher } from 'svelte';
  
  
    export let items = [];
    export let activeTabValue;
  
    const dispatch = createEventDispatcher();

    onMount(() => {
      // Set default tab value
      if (Array.isArray(items) && items.length && items[0].value) {
        activeTabValue = items[0].value;
      }
    });
  
    const handleClick = tabValue => () => {activeTabValue = tabValue; dispatch('changeTab', tabValue);};

    function doNothing() {}
  </script>
  
  <ul>
    {#if Array.isArray(items)}
      {#each items as item}
        <li class={activeTabValue === item.value ? 'active' : ''}>
          <span on:click={handleClick(item.value)} on:keyup={doNothing} role="none">{item.label}</span>
        </li>
      {/each}
    {/if}
  </ul>
  
  <style lang="postcss">
    ul {
      display: flex;
      flex-wrap: wrap;
      padding-left: 0;
      margin-bottom: 0;
      list-style: none;
      border-bottom: 1px solid #dee2e6;
    }
  
    span {
      border: 1px solid transparent;
      border-top-left-radius: 0.25rem;
      border-top-right-radius: 0.25rem;
      display: block;
      padding: 0.5rem 1rem;
      cursor: pointer;
    }
  
    span:hover {
      border-color: #e9ecef #e9ecef #dee2e6;
    }
  
    li.active > span {
      color: #495057;
      background-color: #fff;
      border-color: #dee2e6 #dee2e6 #fff;
    }
  </style>
