<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/getnotes`;
  let renewData = false;
  let context = 'global';
 </script>

<Pages {apiUrl} {renewData} {context}/>
