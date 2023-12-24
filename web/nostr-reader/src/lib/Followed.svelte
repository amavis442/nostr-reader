<script lang="ts">
    import { onMount } from "svelte";
    import Pagination from "./partials/Pagination.svelte";
    import Modal from "./partials/Modal.svelte";
    import TextArea from "./partials/TextArea.svelte";
    import Button from "./partials/Button.svelte";
    import Feeder from "./Feeder.svelte";
    import TextNote from "./TextNote.svelte";

    let pageData = [];
  
    let current_page = 1;
    let from = 1;
    let to = 1;
    let per_page = 1;
    let last_page = 1;
    let total = 0;
    let limit = 60;
    let since = 1; // 1 day
  
    onMount(async () => {
      await refreshView({ page: 1, limit: limit, since: since });
    });
  
    async function refreshView(params) {
      let apiUrl = "/api/getfollownotes";
  
      await fetch(apiUrl, {
        method: "POST",
        body: JSON.stringify(params),
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
  
          current_page = data.current_page;
          from = data.from;
          to = data.to;
          per_page = data.per_page;
          last_page = data.last_page > 10 ? 10 : data.last_page;
          total = data.total;
  
          pageData = data.data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    async function refresh() {
      fetch("/api/sync")
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
          //alert(tabs[currentTab - 1] + ' ' + currentTab)
          refreshView({ page: current_page, limit: limit, since: since });
          document.getElementById("content").scrollTo(0, 0);
          return data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    function blockUser(pubkey) {
      fetch("/api/blockuser", {
        method: "POST",
        body: JSON.stringify({ pubkey: pubkey }),
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
          refreshView({ page: current_page, limit: limit, since: since });
          return data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    function followUser(pubkey) {
      fetch("/api/followuser", {
        method: "POST",
        body: JSON.stringify({ pubkey: pubkey }),
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
          refreshView({ page: current_page, limit: limit, since: since });
          return data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    function unfollowUser(pubkey) {
      fetch("/api/unfollowuser", {
        method: "POST",
        body: JSON.stringify({ pubkey: pubkey }),
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
          refreshView({ page: current_page, limit: limit, since: since })
          return data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    function publish(msg: string, event_id: string) {
      fetch("/api/publish", {
        method: "POST",
        body: JSON.stringify({ msg: msg, event_id: event_id }),
        headers: {
          "Content-Type": "application/json",
        },
      })
        .then((res) => {
          return res.json();
        })
        .then((data) => {
          console.log("Json is ", data);
          return data;
        })
        .catch((err) => {
          console.error("error", err);
        });
    }
  
    let showModal = false;
    let replyToEventId = "";
    async function onPublish(e: Event) {
      const target = e.target as HTMLFormElement;
      const formData = new FormData(target);
  
      const data: { replyText?: string; event_id?: string } = {};
      //@ts-ignore
      for (let field of formData) {
        const [key, value] = field;
        data[key] = value;
      }
  
      publish(data.replyText, data.event_id);
      showModal = false;
      replyToEventId = "";
    }
  </script>
  
  <Modal bind:showModal>
    <h2 slot="header">Create Note</h2>
  
    <form on:submit|preventDefault={onPublish}>
      <input type="hidden" name="event_id" value={replyToEventId} />
      <p><TextArea id="replyText" rows="15" cols="30" /></p>
      <div class="actions">
        <Button type="submit">Send</Button>
      </div>
    </form>
  </Modal>
  
  <main>
    <Feeder>
      <slot>
        <div class="flex flex-col bg-white p-2 rounded-lg m-2">
          <button on:click={refresh} class="btn btn-blue">Sync</button>
          <select
            id="limit"
            bind:value={limit}
            on:change={() =>
              refreshView({ page: current_page, limit: limit, since: since })}
            class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
          >
            {#each [10, 15, 20, 25, 30, 60, 90, 120, 150, 180] as limitValue}
              <option value={limitValue}>
                {limitValue}
              </option>
            {/each}
          </select><label for="limit">Items Per Page</label>
  
          <select
            id="since"
            bind:value={since}
            on:change={() =>
              refreshView({ page: current_page, limit: limit, since: since })}
            class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
          >
            {#each [1, 2, 3, 4, 5, 6, 7] as sinceValue}
              <option value={sinceValue}>
                {sinceValue}
              </option>
            {/each}
          </select> <label for="since"> Days (since)</label>
          <button on:click={() => (showModal = true)} class="btn btn-blue">
            Msg
          </button>
  
          {#if total > per_page}
            <Pagination
              {current_page}
              {last_page}
              {per_page}
              {from}
              {to}
              {total}
              on:change={(ev) => {
                refreshView({ page: ev.detail, limit: limit, since: since });
                document.getElementById("content").scrollTo(0, 0);
              }}
            ></Pagination>
          {/if}
        </div>
  
        <ul class="items-center w-full border-hidden" id="content">
          {#each pageData ? pageData : [] as note (note.id)}
            <TextNote
              {note}
              on:followUser={(ev) => followUser(ev.detail)}
              on:unfollowUser={(ev) => unfollowUser(ev.detail)}
              on:blockUser={(ev) => blockUser(ev.detail)}
              on:reply={(ev) => {
                showModal = true;
                replyToEventId = ev.detail.id;
              }}
            ></TextNote>
          {/each}
        </ul>
      </slot>
    </Feeder>
  </main>
  
  <style>
    .btn {
      @apply font-bold py-2 px-4 rounded;
    }
    .btn-blue {
      @apply bg-blue-500 text-white;
    }
    .btn-blue:hover {
      @apply bg-blue-700;
    }
  </style>
  