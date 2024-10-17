import { writable, get } from 'svelte/store'
import type { Note, Page, Paginator } from '../../types'

export const pageData = writable([])

let apiUrl: string

export const paginator = writable<Paginator>({
	current_page: 1,
	from: 1,
	to: 1,
	per_page: 1,
	last_page: 1,
	total: 0,
	limit: 60,
	since: 1, // 1 day
	renew: true,
	maxid: 0,
	context: null
})

export function setApiUrl(url: string) {
	apiUrl = url
}

const elm: null | HTMLElement = document.getElementById('content')

export async function refreshView(params: Page) {
	return await fetch(apiUrl, {
		method: 'POST',
		body: JSON.stringify(params),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			console.log('Json is ', response)

			let maxId = 0
			maxId = params.maxid

			if (params.renew || params.maxid == 0) {
				maxId = response.maxid
			}

			paginator.set({
				current_page: response.current_page,
				from: response.from,
				to: response.to,
				per_page: response.per_page,
				last_page: response.last_page > 10 ? 10 : response.last_page,
				total: response.total,
				limit: response.limit,
				since: response.since,
				renew: response.renew,
				maxid: maxId,
				context: null
			})
			pageData.set(response.data)
		})
		.then(() => {
			//const elm: null|HTMLElement = document.getElementById("content")
			if (elm) {
				elm.scrollTo(0, 0)
			}
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export async function refresh() {
	fetch(`${import.meta.env.VITE_API_LINK}/api/sync`)
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			console.log('Json is ', response)
			const paginatorData = get(paginator)
			refreshView({
				page: paginatorData.current_page,
				limit: paginatorData.limit,
				since: paginatorData.since,
				renew: true,
				maxid: 0,
				context: "page.sync"
			})
			return response
		})
		.then(() => {
			//const elm: null|HTMLElement = document.getElementById("content")
			if (elm) {
				elm.scrollTo(0, 0)
			}
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export function blockUser(pubkey: string) {
	fetch(`${import.meta.env.VITE_API_LINK}/api/blockuser`, {
		method: 'POST',
		body: JSON.stringify({ pubkey: pubkey }),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			const paginatorData = get(paginator)
			refreshView({
				page: paginatorData.current_page,
				limit: paginatorData.limit,
				since: paginatorData.since,
				renew: false,
				maxid: paginatorData.maxid,
				context: null
			})
			return response
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export function followUser(pubkey: string) {
	fetch(`${import.meta.env.VITE_API_LINK}/api/followuser`, {
		method: 'POST',
		body: JSON.stringify({ pubkey: pubkey }),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			const paginatorData = get(paginator)
			refreshView({
				page: paginatorData.current_page,
				limit: paginatorData.limit,
				since: paginatorData.since,
				renew: false,
				maxid: paginatorData.maxid,
				context: null
			})
			return response
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export function unfollowUser(pubkey: string) {
	fetch(`${import.meta.env.VITE_API_LINK}/api/unfollowuser`, {
		method: 'POST',
		body: JSON.stringify({ pubkey: pubkey }),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			const paginatorData = get(paginator)
			refreshView({
				page: paginatorData.current_page,
				limit: paginatorData.limit,
				since: paginatorData.since,
				renew: false,
				maxid: paginatorData.maxid,
				context: null
			})
			return response
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export async function getNewNotesCount(context: string | null): Promise<number> {
	const paginatorData = get(paginator)
	const data = await fetch(`${import.meta.env.VITE_API_LINK}/api/getnewnotescount`, {
		method: 'POST',
		body: JSON.stringify({
			page: paginatorData.current_page,
			limit: paginatorData.limit,
			since: paginatorData.since,
			renew: false,
			maxid: paginatorData.maxid,
			context: context
		}),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			return response.data
		})
		.catch((err) => {
			console.error('error', err)
		})

	return typeof data === 'object' ? 0 : Number(data)
}

export async function getLastSeenId(): Promise<number> {
	const data = await fetch(`${import.meta.env.VITE_API_LINK}/api/getlastseenid`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			return response.data
		})
		.catch((err) => {
			console.error('error', err)
		})

	return typeof data === 'object' ? 0 : Number(data)
}

//Todo: needs same fix as sunc note so only a portion of the view is updated and not the complete view.
export async function publish(msg: string, note: Note | null) {
	await fetch(`${import.meta.env.VITE_API_LINK}/api/publish`, {
		method: 'POST',
		body: JSON.stringify({ msg: msg, event_id: note ? note.event.id : '' }),
		headers: {
			'Content-Type': 'application/json'
		}
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			console.debug('Json is ', response, ' and note is ', note)
			const paginatorData = get(paginator)
			if (response.status == 'ok' && note == null) {
				refreshView({
					page: paginatorData.current_page,
					limit: paginatorData.limit,
					since: paginatorData.since,
					renew: true,
					maxid: paginatorData.maxid,
					context: null
				})
			}

			if (response.status == 'ok' && note != null) {
				console.debug('Refresh after publish: ', note.event.id)
				refreshView({
					page: paginatorData.current_page,
					limit: paginatorData.limit,
					since: paginatorData.since,
					renew: false,
					maxid: paginatorData.maxid,
					context: null
				})
			}
			return response
		})
		.catch((err) => {
			console.error('error', err)
		})
}

export async function syncNote() {
	const paginatorData = get(paginator)
	const currentPageData = get(pageData)
	let ids: Array<string> = [];

	currentPageData.forEach((note) => {
		ids.push(note.event.id)
	})
	//JSON.stringify(ids)

	await refreshView({
		page: paginatorData.current_page,
		limit: paginatorData.limit,
		since: paginatorData.since,
		renew: false,
		maxid: paginatorData.maxid,
		context: "page.refresh",
		ids: ids,
		total: paginatorData.total
	})
}

export async function tranlateContent(text: string) {
	const translateUrl = import.meta.env.VITE_APP_TRANSLATE_URL
	if (translateUrl == '') {
		return 'Translate url not set'
	}
	return await fetch(import.meta.env.VITE_APP_TRANSLATE_URL, {
		method: 'POST',
		body: JSON.stringify({
			q: text,
			source: 'auto',
			target: import.meta.env.VITE_APP_TRANSLATE_LANG,
			format: 'text',
			api_key: ''
		}),
		headers: { 'Content-Type': 'application/json' }
	})
		.then((res) => {
			return res.json()
		})
		.then((response) => {
			return response.translatedText
		})
		.catch((err) => {
			console.error(err)
		})
}
