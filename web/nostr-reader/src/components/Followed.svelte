<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/getfollownotes`;
  let renewData = false;
  let context  = 'follow';
 </script>

<Pages {apiUrl} {renewData} {context}/>
