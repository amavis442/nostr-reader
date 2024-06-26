export type Page = {
	page: number
	limit: number
	since: number
	maxid: number
	renew: boolean
	context?: string | null
	ids?: array|null
	total?: number|null
}

export interface IRefreshView {
	page: number
	limit: number
	since: number
	renew: boolean
	maxid: number
	context: null | string
}

export interface Paginator {
	current_page: number
	from: number
	to: number
	per_page: number
	last_page: number
	total: number
	limit: number
	since: number
	renew: boolean
	maxid: number
	context: null | string
}

export type Profile = {
	pubkey: string
	name: string
	about: string
	picture: string
	website: string
	pip05: string
	lud16: string
	display_name: string
	followed: boolean
	created_at: Date | null | undefined
	updated_at: Date | null | undefined
}

export type Relay = {
	url: string
	read: boolean
	write: boolean
	search: boolean
	created_at: Date | null | undefined
	updated_at?: Date | null | undefined
}

export type Reaction = {
	pubkey: string
	content: string
	current_vote: string
	target_event_id: string
	from_event_id: string
	created_at: Date | null | undefined
	updated_at?: Date | null | undefined
}


export type NostrEvent = {
	id: string
	content: string
	pubkey: string
	kind: int
	content: string
	tags: string[][]
	sig: string
	created_at: int
}

export type NostrNote = {
	event_id: string
	pubkey: string
	kind: int
	event_created_at: int64
	content: string
	tags: string[][]
	sig: string
	reaction?: Array<Reaction>
	created_at: Date | null | undefined
	updated_at?: Date | null | undefined
}

export type Note = {
	bookmark: boolean
	children: Note[] | null
	content: string
	event: NostrEvent
	garbage: boolean
	profile: Profile
	refs: {
		event: NostrEvent[] | null
		profile: Profile[] | null
	}
	tree: int
}