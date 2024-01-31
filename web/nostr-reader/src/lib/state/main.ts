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
  since: 1, // 1 day
  renew: true,
  maxid: 0
});

export function setApiUrl(url: string) {
  apiUrl = url
}

interface IRefreshView {
  page: number,
  limit: number,
  since: number,
  renew: boolean,
  maxid: number
}

export async function refreshView(params: IRefreshView) {
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

      let maxId = 0;
      let total = 0;

      maxId = params.maxid;
      //total = params.total

      if (params.renew || params.maxid == 0) {
        maxId = data.maxid
        //total = data.total
      }

      pageMetaData.set({
        current_page: data.current_page,
        from: data.from,
        to: data.to,
        per_page: data.per_page,
        last_page: data.last_page > 10 ? 10 : data.last_page,
        total: data.total,
        limit: data.limit,
        since: data.since,
        renew: data.renew,
        maxid: maxId
      })

      pageData.update(() => { return data.data });
      return data.data;
    })
    .then(() => {
      let elm: null|HTMLElement = document.getElementById("content")
      if (elm) {
        elm.scrollTo(0, 0);
      } 
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export async function refresh() {
  fetch(`${import.meta.env.VITE_API_LINK}/api/sync`)
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.log("Json is ", data);
      const pageData = get(pageMetaData)
      refreshView({
        page: pageData.current_page,
        limit: pageData.limit,
        since: pageData.since,
        renew: true,
        maxid: 0
      });
      return data;
    })
    .then(() => {
      let elm: null|HTMLElement = document.getElementById("content")
      if (elm) {
        elm.scrollTo(0, 0);
      }    
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function blockUser(pubkey: string) {
  fetch(`${import.meta.env.VITE_API_LINK}/api/blockuser`, {
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
      const pageData = get(pageMetaData)
      refreshView({
        page: pageData.current_page,
        limit: pageData.limit,
        since: pageData.since,
        renew: false,
        maxid: pageData.maxid
      });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function followUser(pubkey: string) {
  fetch(`${import.meta.env.VITE_API_LINK}/api/followuser`, {
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
      const pageData = get(pageMetaData)
      refreshView({
        page: pageData.current_page,
        limit: pageData.limit,
        since: pageData.since,
        renew: false,
        maxid: pageData.maxid
      });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export function unfollowUser(pubkey: string) {

  fetch(`${import.meta.env.VITE_API_LINK}/api/unfollowuser`, {
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
      const pageData = get(pageMetaData)
      refreshView({
        page: pageData.current_page,
        limit: pageData.limit,
        since: pageData.since,
        renew: false,
        maxid: pageData.maxid
      });
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}


export async function getNewNotesCount(): Promise<number> {
  const pageData = get(pageMetaData)
  let data = await fetch(`${import.meta.env.VITE_API_LINK}/api/getnewnotescount`, {
    method: "POST",
    body: JSON.stringify({
      page: pageData.current_page,
      limit: pageData.limit,
      since: pageData.since,
      renew: false,
      maxid: pageData.maxid
    }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      return data.data;
    })
    .catch((err) => {
      console.error("error", err);
    });

  return typeof data === 'object' ? 0 : Number(data)
}

export async function getLastSeenId(): Promise<number> {
  const pageData = get(pageMetaData)
  let data = await fetch(`${import.meta.env.VITE_API_LINK}/api/getlastseenid`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      return data.data;
    })
    .catch((err) => {
      console.error("error", err);
    });

  return typeof data === 'object' ? 0 : Number(data)
}

//Todo: needs same fix as sunc note so only a portion of the view is updated and not the complete view.
export async function publish(msg: string, note: any) {
  await fetch(`${import.meta.env.VITE_API_LINK}/api/publish`, {
    method: "POST",
    body: JSON.stringify({ msg: msg, event_id: note ? note.event.id : "" }),
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      console.debug("Json is ", data);
      const pageData = get(pageMetaData);
      if (note == "") {
        refreshView({
          page: pageData.current_page,
          limit: pageData.limit,
          since: pageData.since,
          renew: true,
          maxid: pageData.maxid
        });
      }

      if (note != "") {
        console.debug("Refresh after publish: ", note.event.id);
        refreshView({
          page: pageData.current_page,
          limit: pageData.limit,
          since: pageData.since,
          renew: false,
          maxid: pageData.maxid
        });
      }
      return data;
    })
    .catch((err) => {
      console.error("error", err);
    });
}

export async function syncNote() {
  const pageData = get(pageMetaData)
  await refreshView({
    page: pageData.current_page,
    limit: pageData.limit,
    since: pageData.since,
    renew: false,
    maxid: pageData.maxid
  });
}

export async function tranlateContent(text: string) {
  let translateUrl = import.meta.env.VITE_APP_TRANSLATE_URL;
  if (translateUrl == "") {
    return "Translate url not set";
  }
  return await fetch(import.meta.env.VITE_APP_TRANSLATE_URL, {
    method: "POST",
    body: JSON.stringify({
      q: text,
      source: "auto",
      target: import.meta.env.VITE_APP_TRANSLATE_LANG,
      format: "text",
      api_key: "",
    }),
    headers: { "Content-Type": "application/json" },
  })
    .then((res) => {
      return res.json();
    })
    .then((data) => {
      return data.translatedText;
    })
    .catch((err) => {
      console.error(err);
    });
}