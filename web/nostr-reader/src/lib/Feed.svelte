<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/events`;
  let renewData = true;
 </script>

<Pages {apiUrl} {renewData}/>
