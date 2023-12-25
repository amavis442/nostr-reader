
import { writable, get } from 'svelte/store';
export let pageData = writable([]);


let apiUrl: string;

export let pageMetaData = writable({
  current_page: 1,
  from: 1,
  to: 1,
  per_page: 1,
  last_page: 1,
  total: 0,
  limit: 60,
  since: 1 // 1 day
});

export function setApiUrl(url: string) {
  apiUrl = url
}

export async function refreshView(params) {
  return await fetch(apiUrl, {
    method: "POST",
    body: JSON.stringify(params),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);

      pageMetaData.set({
          current_page: data.current_page,
          from: data.from,
          to: data.to,
          per_page: data.per_page,
          last_page: data.last_page > 10 ? 10 : data.last_page,
          total: data.total,
          limit: data.limit,
          since: data.since
      })

      pageData.update(() => { return data.data });
      return data.data;
    })
    //.then(() => (document.getElementById("content").scrollTo(0, 0)))
    .catch((err) => {
      console.error("error", err);
    });
}

export async function refresh() {
  fetch(`${import.meta.env.VITE_PREVIEW_LINK}/api/sync`)
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      const pageData = get(pageMetaData)
      refreshView({ page: pageData.current_page, limit: pageData.limit, since: pageData.since });
      return data;
    })
    .then(() => (document.getElementById("content").scrollTo(0, 0)))
    .catch((err) => {
      console.error("error", err);
    });
}

export function blockUser(pubkey: string) {
  fetch(`${import.meta.env.VITE_PREVIEW_LINK}/api/blockuser`, {
    method: "POST",
    body: JSON.stringify({ pubkey: pubkey }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      const pageData = get(pageMetaData)
      refreshView({ page: pageData.current_page, limit: pageData.limit, since: pageData.since });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function followUser(pubkey: string) {
  fetch(`${import.meta.env.VITE_PREVIEW_LINK}/api/followuser`, {
    method: "POST",
    body: JSON.stringify({ pubkey: pubkey }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      const pageData = get(pageMetaData)
      refreshView({ page: pageData.current_page, limit: pageData.limit, since: pageData.since });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function unfollowUser(pubkey: string) {
  fetch(`${import.meta.env.VITE_PREVIEW_LINK}/api/unfollowuser`, {
    method: "POST",
    body: JSON.stringify({ pubkey: pubkey }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      const pageData = get(pageMetaData)
      refreshView({ page: pageData.current_page, limit: pageData.limit, since: pageData.since });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function publish(msg: string, event_id: string) {
  fetch(`${import.meta.env.VITE_PREVIEW_LINK}/api/publish`, {
    method: "POST",
    body: JSON.stringify({ msg: msg, event_id: event_id }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}
