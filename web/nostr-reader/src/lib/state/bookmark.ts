import {refreshView, pageData, pageMetaData} from './main.ts';
import { writable, get } from 'svelte/store';


export function addBookmark(eventID: string) {
    fetch(`${import.meta.env.VITE_API_LINK}/api/bookmark`, {
      method: "POST",
      body: JSON.stringify({ event_id: eventID }),
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
  
  export function removeBookmark(eventID: string) {
    fetch(`${import.meta.env.VITE_API_LINK}/api/removebookmark`, {
      method: "POST",
      body: JSON.stringify({ event_id: eventID }),
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