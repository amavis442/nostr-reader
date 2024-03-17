<script lang="ts">
	import { onMount } from 'svelte'
	import { toHtml, findLink } from '../../lib/util/html'
	import { tranlateContent } from '../../lib/state/main'
	import Preview from './Preview/Preview.svelte'
	import { createEventDispatcher } from 'svelte'

	import ReadMore from './Readmore/ReadMore.svelte';
	import type { Note, Profile } from '../../types'

	export let note: Note

	const dispatch = createEventDispatcher()

	let imgUrls: string | any[] = []
	let hasImgUrls = false
	let content: string = ''

	onMount(() => {
		imgUrls = findLink(note.event.content)

		if (imgUrls && imgUrls.length > 0) {
			hasImgUrls = true
		}

		content = processRefs(note)
		content = toHtml(content)
	})

	let translatedContent: string = ''
	async function tranlate() {
		translatedContent = await tranlateContent(note.event.content)
	}
	function doNothing() {}

	function processRefs(note: Note): string {
		const eventPrefix =
			"<div class='rounded-2xl border border-solid border-medium bg-indigo-300 overflow-hidden p-1 m-2' id='noteid'> <i class='fa-regular fa-note-sticky'></i> "
		const eventAffix = '</div>'
		const profilePrefix =
			"<span class='rounded-2xl border border-solid border-medium bg-indigo-300 overflow-hidden p-1' id='profileid'><i class='fa-solid fa-user'></i> "
		const profileAffix = '</span>'
		let content: string = note.content

		if (Object.keys(note.refs.event).length == 0 && Object.keys(note.refs.profile).length == 0) {
			// The leftovers without present data to replace them
			content = note.content
			content = content.replaceAll('[~[', profilePrefix.replace("id='profileid'", ''))
			content = content.replaceAll(']~]', profileAffix)
			content = content.replaceAll('[~~[', eventPrefix.replace("id='profileid'", ''))
			content = content.replaceAll(']~~]', eventAffix)

			return content
		}



		if (Object.keys(note.refs.event).length > 0) {
			const eventKeys = Object.keys(note.refs.event)
			for (let i = 0; i < eventKeys.length; i++) {
				let ref = note.refs.event[eventKeys[i]]
				content = content.replaceAll(
					'[~~[' + eventKeys[i] + ']~~]',
					eventPrefix.replace('noteid', 'note_' + ref.id) +
						ref.content.substring(0, 100) +
						' (.....)' +
						eventAffix
				)
			}
		}
		if (Object.keys(note.refs.profile).length > 0) {
			const profileKeys = Object.keys(note.refs.profile)

			for (let i = 0; i < profileKeys.length; i++) {
				let ref = note.refs.profile[profileKeys[i]]
				content = content.replaceAll(
					'[~[' + profileKeys[i] + ']~]',
					profilePrefix.replace('profileid', 'profile_' + ref.pubkey) + ref.name + profileAffix
				)
			}
		}

		// The leftovers without present data to replace them
		content = content.replaceAll('[~[', profilePrefix.replace("id='profileid'", ''))
		content = content.replaceAll(']~]', profileAffix)
		content = content.replaceAll('[~~[', eventPrefix.replace("id='profileid'", ''))
		content = content.replaceAll(']~~]', eventAffix)


		return content
	}

	function textEvent(event: MouseEvent) {
		let id = (<HTMLElement>event.target).id

		if (id) {
			if (id.indexOf('profile_', 0) != -1) {
				let profileId: string = id.replace('profile_', '')
				let profile: Profile = note.refs.profile[profileId]
				dispatch('profileInfo', { profile: profile })
			}

			if (id.indexOf('note_', 0) != -1) {
				let eventId: string = id.replace('note_', '')
				console.debug('Got an id: ' + eventId)
				let noteRef: Event = note.refs.event[eventId]
				console.debug(noteRef)
				dispatch('noteInfo', { note: noteRef })
			}
		}
	}
</script>

<span class="text-black text-md font-medium break-words">
	<p on:click={textEvent} role="none">
		<ReadMore textContent={content} maxWords={30} />
	</p>
	{#if import.meta.env.VITE_APP_TRANSLATE_URL && import.meta.env.VITE_APP_TRANSLATE_LANG}
		<button on:click={tranlate} class="p-1 m-2" title="Translate"
			>Translate to ({import.meta.env.VITE_APP_TRANSLATE_LANG})</button
		>
		{#if translatedContent != ''}
			<div
				id="translateContent_{note.event.id}"
				class="rounded-2xl border border-solid border-medium bg-white p-4 mt-2 mb-2"
			>
				{translatedContent}
			</div>
		{/if}
	{/if}
</span>
{#if hasImgUrls}
	{#each imgUrls as s, outerIndex}
		{#if outerIndex % 3 === 0}
			<div
				class="mt-4 flex flex-cols-2 gap-4 bg-bg_color"
				on:click={(e) => e.stopPropagation()}
				on:keyup={doNothing}
				role="button"
				tabindex="0"
			>
				{#each imgUrls as imgUrl, i}
					{#if i >= outerIndex && i < outerIndex + 3}
						<Preview endpoint={`${import.meta.env.VITE_API_LINK}/api/preview/link`} url={imgUrl} />
					{/if}
				{/each}
			</div>
		{/if}
	{/each}
{/if}
