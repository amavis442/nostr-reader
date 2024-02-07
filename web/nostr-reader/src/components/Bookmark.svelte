<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/getbookmarked`;
  let renewData = false;
  let context = 'bookmark';
 </script>

<Pages {apiUrl} {renewData} {context} />
