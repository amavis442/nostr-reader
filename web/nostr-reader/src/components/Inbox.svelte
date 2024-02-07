<script lang="ts">
  import Pages from "./Pages.svelte";
  let apiUrl = `${import.meta.env.VITE_API_LINK}/api/getinbox`;
  let renewData = false;
  let context  = 'inbox';
 </script>

<Pages {apiUrl} {renewData} {context}/>
