<script>
  import { paginator } from '../../lib/state/main';
  import { createEventDispatcher } from "svelte";

  const dispatch = createEventDispatcher();

  function range(size, startAt = 0) {
    return [...Array(size).keys()].map((i) => i + startAt);
  }

  function changePage(page) {
    if (page !== $paginator.current_page) {
      dispatch("change", page);
    }
  }
</script>

<p>
  Page <code>{$paginator.current_page}</code> of
  <code>{$paginator.last_page}</code>
  (<code>{$paginator.from + 1}</code> - <code>{$paginator.to}</code> on
  <code>{$paginator.total}</code>
  items (per page {$paginator.per_page}))
</p>

<nav class="pagination">
  <ul>
    <li class={$paginator.current_page === 1 ? "disabled" : ""}>
      <a
        href={"#"}
        on:click={() => changePage($paginator.current_page - 1)}
        aria-label="Previous"
        class="mx-1 flex h-9 w-9 items-center justify-center rounded-full border border-blue-gray-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300"
      >
        <span aria-hidden="true">«</span>
      </a>
    </li>
    {#each range($paginator.last_page, 1) as page}
      <li class={page === $paginator.current_page ? "active" : ""}>
        <a
          href={"#"}
          on:click={() => changePage(page)}
          class="mx-1 flex h-9 w-9 items-center justify-center rounded-full {page ===
          $paginator.current_page
            ? 'bg-blue-600 hover:bg-blue-700 p-0 text-sm text-white shadow-md'
            : 'border border-blue-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300'}"
          >{page}</a
        >
      </li>
    {/each}
    <li
      class={$paginator.current_page === $paginator.last_page
        ? "disabled"
        : ""}
    >
      <a
        href={"#"}
        on:click={() => changePage($paginator.current_page + 1)}
        aria-label="Next"
        class="mx-1 flex h-9 w-9 items-center justify-center rounded-full border border-blue-gray-100 bg-transparent p-0 text-sm text-blue-gray-500 transition duration-150 ease-in-out hover:bg-light-300"
      >
        <span aria-hidden="true">»</span>
      </a>
    </li>
  </ul>
</nav>

<style lang="postcss">
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
    padding: 0.5rem 0.75rem;
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
