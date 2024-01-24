<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/getbookmarked`;
 </script>

<Pages {apiUrl} />
