<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_PREVIEW_LINK}/api/events`;
 </script>

<Pages {apiUrl} />
