<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from "svelte";
  import { get, writable, type Writable } from "svelte/store";
  import placeholder from './assets/profile-picture.jpg';



  let page = writable([]);
  let test = []
  onMount(async () => {
    await refreshView();
  });

async function refreshView() {
  const json = await fetch("/api/follow", {
      method: "POST",
      //body: JSON.stringify({ pubkey: note.pubkey }),
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

      console.log(json)
      test = json
}


  async function refresh() {
    fetch("/api/getnext")
  }

  async function blockUser(pubkey) {
    await fetch("/api/blockuser", {
      method: "POST",
      body: JSON.stringify({ pubkey: pubkey }),
      headers: {
        "Content-Type": "application/json",
      }
    }).then((res) => {
        return res.json();
      })
      .then((data) => {
        console.log("Json is ", data);
        refreshView();
        return data;
      })
      .catch((err) => {
        console.error("error", err);
      });
  }
</script>

<main>
  <button on:click="{refresh}" class="btn btn-blue">Sync</button>
  <button on:click="{refreshView}" class="btn btn-blue">refreshView</button>
  
  <div class="p-10 mb-10">  
    {#each test ? test : [] as note (note.id)}
    <div class="rounded-t-lg overflow-hidden border-t border-l border-r border-gray-400 p-10 flex justify-center">
      <div class="max-w-sm w-full lg:max-w-full lg:flex">
        <div class="h-48 lg:h-auto lg:w-48 flex-none bg-cover rounded-t lg:rounded-t-none lg:rounded-l text-center overflow-hidden" style="background-image: url('https://images.alphacoders.com/188/188121.jpg')" title="Mountain">
        </div>
        <div class="border-r border-b border-l border-gray-400 lg:border-l-0 lg:border-t lg:border-gray-400 bg-white rounded-b lg:rounded-b-none lg:rounded-r p-4 flex flex-col justify-between leading-normal">
          <div class="mb-8">
            <p class="text-gray-700 text-base">{ note.content }</p>
          </div>
          <div class="flex items-center">
            <img class="w-10 h-10 rounded-full mr-4" src="{ note.picture ? note.picture : placeholder}" alt="Placeholder" title="{ note.about ? note.about : ''}">
            <div class="text-sm">
              <p class="text-gray-900 leading-none">{ note.name.slice(0, 20) } <button on:click="{blockUser(note.pubkey)}">Block</button></p>
              <p class="text-gray-600"><small>{ (new Date(note.created_at  * 1000)).toLocaleString('nl-NL') }</small></p>
            </div>
          </div>
        </div>
      </div>
    </div>
    {/each}
  </div>
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
