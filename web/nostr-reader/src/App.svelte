<script lang="ts">
  import { Router, Link, Route } from "svelte-routing";
  import Feed from "./routes/Feed.svelte";
  import Followed from "./routes/Followed.svelte";
  import Inbox from "./routes/Inbox.svelte";
  import Bookmark from "./routes/Bookmark.svelte";
  import Account from "./routes/settings/Account.svelte";
  import Relay from "./routes/settings/Relay.svelte";
  import Profiles from "./routes/Profiles.svelte";
  import "@fortawesome/fontawesome-free/css/fontawesome.css";
  import "@fortawesome/fontawesome-free/css/solid.css";
  import Toasts from "./components/partials/Toast/Toasts.svelte";
  import { Modals, closeModal } from "svelte-modals";
  export let url = "";

</script>

<Toasts />

<Router url="{url}">
  <div class="flex justify-center">
    <div
      class="flex flex-col pt-6 items-center pl-4 space-y-3 pb-5 xl:w-1/12 md:w-3/12 sm:w-full"
    >
      <nav>
        <p class="nav-p">
          <Link to="/" title="Shows all notes from everybody you follow" let:active>
            <span class="{active ? 'selected' : ''}">Home feed</span>
          </Link>
        </p>
        <p class="nav-p {url === '/global' ? 'selected' : ''}" >
          <Link to="/global" title="Global feed" class="flex" let:active
            >
            <span class="{active ? 'selected' : ''}">
            Global Feed
            </span>
          </Link>
        </p>
        <p class="nav-p {url === '/inbox' ? 'selected' : ''}">
          <Link to="/inbox" title="Your replies and inbox" let:active>
            <span class="{active ? 'selected' : ''}">Own Replies</span>
            </Link>
        </p>
        <p class="nav-p {url === '/bookmark' ? 'selected' : ''}">
          <Link to="/bookmark" title="Bookmarked" let:active>
            <span class="{active ? 'selected' : ''}">Bookmarked</span>
            </Link>
        </p>
        <p class="nav-p {url === '/account' ? 'selected' : ''}">
          <Link to="account" title="Your account data" let:active>
            <span class="{active ? 'selected' : ''}">Account</span>
          </Link>
        </p>
        <p class="nav-p {url === '/relay' ? 'selected' : ''}">
          <Link to="relay" title="Your relays" let:active>
            <span class="{active ? 'selected' : ''}">Relays</span>
          </Link>
        </p>
        <p class="nav-p {url === '/profiles' ? 'selected' : ''}">
          <Link to="profiles" title="Your followed profiles" let:active>
            <span class="{active ? 'selected' : ''}">Followed</span>
          </Link>
        </p>
      </nav>
    </div>

    <main class="xl:w-8/12 md:w-9/12 sm:w-full overflow-y-auto">
      <Route path="/" component="{Followed}" />
      <Route path="global" component="{Feed}" />
      <Route path="inbox" component="{Inbox}" />
      <Route path="bookmark" component="{Bookmark}" />
      <Route path="account" component="{Account}" />
      <Route path="relay" component="{Relay}" />
      <Route path="profiles" component="{Profiles}" />
    </main>
  </div>
</Router>

<Modals>
  <div
    slot="backdrop"
    class="backdrop"
    on:click={closeModal}
    on:keyup={closeModal}
    role="none"
  />
</Modals>

<style lang="postcss">
  .backdrop {
    position: fixed;
    top: 0;
    bottom: 0;
    right: 0;
    left: 0;
    background: rgba(0, 0, 0, 0.5);
  }

  .selected {
    text-decoration-line: underline;
  }
</style>
