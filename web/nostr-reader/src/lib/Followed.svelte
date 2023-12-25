<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_PREVIEW_LINK}/api/getfollownotes`;
 </script>

<Pages {apiUrl} />
