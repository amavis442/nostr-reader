<script lang="ts">
  import { onMount } from "svelte";
  import Button from "./partials/Button.svelte";
  import Text from "./partials/Text.svelte";
  import { addToast } from "./partials/Toast/toast";

  let name: string;
  let about: string;
  let picture: string;
  let nip05: string;
  let website: string;
  let displayname: string;

  /**
   * @see https://www.thisdot.co/blog/handling-forms-in-svelte
   * @param e
   */
  function onSubmit() {
    fetch(`${import.meta.env.VITE_API_LINK}/api/notifications`, {
      method: "POST",
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

    addToast({
      message: "Account updated!",
      type: "success",
      dismissible: true,
      timeout: 3000,
    });
  }

  function getMetaData() {
    fetch(`${import.meta.env.VITE_API_LINK}/api/getmetadata`, {
      method: "POST",
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

  onMount(async () => {});
</script>

<div class="xl:w-8/12 lg:w-10/12 md:w-10/12 sm:w-full">
  <div
    class="block p-6 rounded-lg shadow-lg bg-white w-full ml-6 mt-6 bg-blue-200"
  >
    <form on:submit|preventDefault={onSubmit}>
      <div class="flex justify-end w-full gap-2">
        <div class="col-2">
          <Button type="submit">Submit</Button>
        </div>
      </div>
    </form>
  </div>
</div>
