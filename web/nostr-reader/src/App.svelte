<script lang="ts">
  import { Router, Link, Route } from "svelte-routing";
  import Feed from "./routes/Feed.svelte";
  import Followed from "./routes/Followed.svelte";
  import Inbox from "./routes/Inbox.svelte";
  import Bookmark from "./routes/Bookmark.svelte";
  import Account from "./routes/Account.svelte";
  import "@fortawesome/fontawesome-free/css/fontawesome.css";
  import "@fortawesome/fontawesome-free/css/solid.css";
  import Toasts from "./lib/partials/Toast/Toasts.svelte";
  import { Modals, closeModal } from "svelte-modals";
  export let url = "";

</script>

<Toasts />

<Router url="{url}">
  <div class="flex h-screen w-screen m-auto justify-center">
    <header
      class="mt-6 items-center pl-4 border-gray-600 border-b space-y-3 pb-5 xl:w-2/12 md:w-3/12 sm:w-full"
    >
      <nav>
        <p class="nav-p {url === '/' ? 'selected' : ''}">
          <Link to="/" title="Global feed" class="flex"
            >
            <div class="justify-end w-full">
            Global
          </div>
          </Link>
        </p>
        <p class="nav-p {url === '/followed' ? 'selected' : ''}">
          <Link to="/followed" title="Contacts that you follow">
            Following
            </Link>
        </p>
        <p class="nav-p {url === '/inbox' ? 'selected' : ''}">
          <Link to="/inbox" title="Your replies and inbox">
            Own Replies
            </Link>
        </p>
        <p class="nav-p {url === '/bookmark' ? 'selected' : ''}">
          <Link to="/bookmark" title="Bookmarked">
            Bookmarked
            </Link>
        </p>
        <p class="nav-p {url === '/account' ? 'selected' : ''}">
          <Link to="account" title="Your account data">Account</Link>
        </p>
      </nav>
    </header>

    <main class="xl:w-6/12 md:w-9/12 sm:w-full overflow-y-auto">
      <Route path="/" component="{Feed}" />
      <Route path="global" component="{Feed}" />
      <Route path="followed" component="{Followed}" />
      <Route path="inbox" component="{Inbox}" />
      <Route path="bookmark" component="{Bookmark}" />
      <Route path="account" component="{Account}" />
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

<style>
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
