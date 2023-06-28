<script>
    export let current_page;
    export let last_page;
    export let per_page;
    export let from;
    export let to;
    export let total;
  
    import { createEventDispatcher } from 'svelte';
  
    const dispatch = createEventDispatcher();
  
    function range(size, startAt = 0) {
      return [...Array(size).keys()].map(i => i + startAt);
    }
  
    function changePage(page) {
      if (page !== current_page) {
        dispatch('change', page);
      }
    }
  </script>
  
  <p>
    Page <code>{current_page}</code> of <code>{last_page}</code> (<code>{from + 1}</code> - <code>{to}</code> on <code>{total}</code> items (per page {per_page}))
  </p>
  
  <nav class="pagination">
    <ul>
      <li class="{current_page === 1 ? 'disabled' : ''}">
        <a href={'#'} on:click="{() => changePage(current_page - 1)}" aria-label="Previous" class="mx-1 flex h-9 w-9 items-center justify-center rounded-full border border-blue-gray-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300">
          <span aria-hidden="true">«</span>
        </a>
      </li>
      {#each range(last_page, 1) as page}
        <li class="{page === current_page ? 'active': ''}">
          <a href={'#'} on:click="{() => changePage(page)}" 
            class="mx-1 flex h-9 w-9 items-center justify-center rounded-full {page === current_page ? 'bg-gradient-to-tr from-pink-600 to-pink-400 p-0 text-sm text-white shadow-md shadow-pink-500/20 transition duration-150 ease-in-out': 'border border-blue-gray-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300'}">{page}</a>
        
        </li>
      {/each}
      <li class="{current_page === last_page ? 'disabled' : ''}">
        <a href={'#'} on:click="{() => changePage(current_page + 1)}" aria-label="Next" class="mx-1 flex h-9 w-9 items-center justify-center rounded-full border border-blue-gray-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300">
          <span aria-hidden="true">»</span>
        </a>
      </li>
    </ul>
  </nav>

  <style>
    .pagination {
      display: flex;
      justify-content: center;
    }
    .pagination ul {
      display: flex;
      padding-left: 0;
      list-style: none;
    }
    .pagination li a {
      position: relative;
      display: block;
      padding: .5rem .75rem;
      margin-left: -1px;
      line-height: 1.25;
      background-color: #fff;
      border: 1px solid #dee2e6;
    }
    .pagination li.active a {
      color: #fff;
      background-color: #007bff;
      border-color: #007bff;
    }
    .pagination li.disabled a {
      color: #6c757d;
      pointer-events: none;
      cursor: auto;
      border-color: #dee2e6;
    }
  </style>
