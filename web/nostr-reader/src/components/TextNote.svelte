<script lang="ts">
	// @ts-nocheck

	import { createEventDispatcher } from 'svelte'
	import NoteContent from './partials/NoteContent.svelte'

	import placeholder from '../assets/profile-picture.jpg'
	import Icon from 'svelte-icons-pack/Icon.svelte'
	import FaSolidInfoCircle from 'svelte-icons-pack/fa/FaSolidInfoCircle'
	import FaSolidUserMinus from 'svelte-icons-pack/fa/FaSolidUserMinus'
	import FaSolidUserPlus from 'svelte-icons-pack/fa/FaSolidUserPlus'

	import FaBookmark from 'svelte-icons-pack/fa/FaBookmark'
	import FaSolidBookmark from 'svelte-icons-pack/fa/FaSolidBookmark'

	import FaFolder from 'svelte-icons-pack/fa/FaFolder'
	import FaFolderOpen from 'svelte-icons-pack/fa/FaFolderOpen'

	import FaSolidBan from 'svelte-icons-pack/fa/FaSolidBan'
	import FaCommentDots from 'svelte-icons-pack/fa/FaCommentDots'
	import FaSolidSync from 'svelte-icons-pack/fa/FaSolidSync'
	import FaSolidLongArrowAltUp from 'svelte-icons-pack/fa/FaSolidLongArrowAltUp'
	import type { Note, Profile } from '../types'

	const dispatch = createEventDispatcher()
	export let note: Note

	function followUser(pubkey: string) {
		dispatch('followUser', pubkey)
		note.profile.followed = true
	}

	function unfollowUser(pubkey: string) {
		dispatch('unfollowUser', pubkey)
		note.profile.followed = false
	}

	function addBookmark(eventID: string) {
		dispatch('addBookmark', eventID)
		note.bookmark = true
	}

	function removeBookmark(eventID: string) {
		dispatch('removeBookmark', eventID)
		note.bookmark = false
	}

	function blockUser(pubkey: string) {
		if (confirm('Block user?') == true) {
			dispatch('blockUser', pubkey)
		}
	}

	function reply(note: Note) {
		dispatch('reply', note)
	}

	function info(note: Note) {
		dispatch('info', note)
	}
	function syncnote(note: Note) {
		dispatch('syncNote', note)
	}

	function gotoTopOfPage(note: Note) {
		dispatch('topPage', note)
	}

	let repliesExpanded = false
	function toggleReplies() {
		repliesExpanded = !repliesExpanded
	}

	function normalizeName(profile: Profile): string {
		return (profile ? (profile.name ? profile.name : note.event.pubkey) : note.event.pubkey).slice(
			0,
			profile && profile.name.length < 50 ? profile.name.length : 20
		)
	}

	// For tailwind to recognise all the colors to include
	let borderColor = 'border-blue-100'
	switch (note.tree) {
		case 0:
			borderColor = 'border-blue-200'
			break
		case 1:
			borderColor = 'border-blue-300'
			break
		case 2:
			borderColor = 'border-blue-400'
			break
		case 3:
			borderColor = 'border-blue-500'
			break
		default:
			borderColor = 'border-blue-100'
	}

	function align() {
		if (note.tree == 0) return ''
	}

	function firstBlock() {
		if (note.tree === 0) {
			return 'border-l-4 border-t-2 ' + borderColor
		}
		return ''
	}

	function childBlock() {
		if (note.tree > 0) {
			return 'border-l-4 border-t-2 ' + borderColor
		}

		return ''
	}

	$: followed = note.profile.followed
	$: bookmarked = note.bookmark
</script>

{#if note && note.event.kind == 1}
	<li>
		<div class="flex flex-col items-top p-2 w-full overflow-hidden mb-2">
			<div
				class="flex flex-col overflow-y-auto bg-slate-600 rounded-lg p-1 {firstBlock()} {$$props[
					'class'
				]
					? $$props['class']
					: ''}"
			>
				<div
					id={note.id}
					class="flex w-full min-h-full {align()} items-top gap-2 mb-2 overflow-y-auto bg-slate-200 rounded-lg p-1 {childBlock()}"
				>
					<div
						on:keyup={() => console.log('keyup')}
						class="w-16 mr-2 max-w-min min-w-fit"
						tabindex="0"
						role="button"
					>
						<img
							class="w-14 h-14 rounded-full {followed ? 'border-2 border-green-800' : ''}"
							src={note.profile.picture != '' ? note.profile.picture : placeholder}
							title={note.profile.about ? note.profile.about : ''}
							alt={note.event.pubkey.slice(0, 10)}
						/>
					</div>

					<div class="flex flex-col w-full">
						<div class="px-2">
							<div class="flex gap-2 h-14 w-full py-2 border-b border-gray-400">
								<div class="text-left order-first w-6/12">
									<strong class="text-black text-sm font-medium">
										<span title={note.event.pubkey}>{normalizeName(note.profile)}</span>
										{#if followed}
											<i class="fa-solid fa-bookmark" />
										{/if}
										<small class="text-gray"
											>{new Date(note.event.created_at * 1000).toLocaleString('nl-NL')}</small
										>
									</strong>
								</div>

								<div class="text-right order-last md:w-6/12">
									<div class="text-right">
										<div class="flex content-normal justify-end">
											<div>
												{#if followed}
													<button on:click={unfollowUser(note.event.pubkey)} title="unfollow"
														><Icon src={FaSolidUserMinus} size="24" color="white" /></button
													>
												{:else}
													<button on:click={followUser(note.event.pubkey)} title="follow"
														><Icon src={FaSolidUserPlus} size="24" color="white" /></button
													>
												{/if}
											</div>

											<div>
												{#if bookmarked}
													<button on:click={removeBookmark(note.event.id)} title="remove bookmark"
														><Icon src={FaSolidBookmark} size="24" color="white" /></button
													>
												{:else}
													<button on:click={addBookmark(note.event.id)} title="add bookmark"
														><Icon src={FaBookmark} size="24" color="white" /></button
													>
												{/if}
											</div>

											<div>
												<button on:click={reply(note)} title="reply"
													><Icon src={FaCommentDots} size="24" color="white" /></button
												>
											</div>
											<div>
												<button on:click={info(note)} title="info"
													><Icon src={FaSolidInfoCircle} size="24" color="white" /></button
												>
											</div>
											<div>
												<button on:click={syncnote(note)} title="sync note"
													><Icon src={FaSolidSync} size="24" color="white" /></button
												>
											</div>
											<div>
												<button on:click={blockUser(note.event.pubkey)} title="block"
													><Icon src={FaSolidBan} size="24" color="white" /></button
												>
											</div>
											<div>
												<button on:click={gotoTopOfPage(note)} title="block"
													><Icon src={FaSolidLongArrowAltUp} size="24" color="white" /></button
												>
											</div>
										</div>
									</div>
								</div>
							</div>
						</div>

						<div class="p-2 w-11/12">
							<div class="text-left w-full max-w-max break-words items-top">
								<NoteContent {note} on:profileInfo on:noteInfo></NoteContent>
							</div>
						</div>

						<div class="w-full">
							<p class="mt-4 flex space-x-8 w-full p-1">
								<span>
									{#if note.children && Object.keys(note.children).length > 0}
										<button type="button" on:click={toggleReplies} class="">
											{#if repliesExpanded}
												<Icon src={FaFolderOpen} size="24" className="inline" />
											{:else}
												<Icon src={FaFolder} size="24" className="inline" />
											{/if}
											<small>({Object.keys(note.children).length})</small>
										</button>
									{/if}
								</span>
							</p>
						</div>
					</div>
				</div>
				{#if repliesExpanded}
					{#if note.children && Object.keys(note.children).length > 0}
						<ul>
							{#each Object.values(note.children) as note (note.event.id)}
								<li>
									<!--on:blockUser is required here so that the event is forwarded-->
									<!--https://dev.to/mohamadharith/workaround-for-bubbling-custom-events-in-svelte-3khk-->
									<svelte:self
										{note}
										on:followUser
										on:unfollowUser
										on:blockUser
										on:reply
										on:info
										on:topPage
										on:profileInfo
										on:noteInfo
									/>
								</li>
							{/each}
						</ul>
					{/if}
				{/if}
			</div>
		</div>
	</li>
{/if}

<style lang="postcss">
	.border-indigo-100 {
		border-color: rgb(224 231 255);
	}
	.border-indigo-200 {
		border-color: rgb(199 210 254);
	}
	.border-indigo-300 {
		border-color: rgb(165 180 252);
	}
	.border-indigo-400 {
		border-color: rgb(129 140 248);
	}
	.border-indigo-500 {
		border-color: rgb(99 102 241);
	}
	.border-indigo-600 {
		border-color: rgb(79 70 229);
	}
	.border-indigo-700 {
		border-color: rgb(67 56 202);
	}
	.border-indigo-800 {
		border-color: rgb(55 48 163);
	}
	.border-indigo-900 {
		border-color: rgb(49 46 129);
	}

	button {
		@apply p-1 bg-blue-600 hover:bg-blue-700 rounded ml-1 mr-1 text-white;
	}

	include-color-blue-100 {
		@apply border-blue-100;
	}

	include-color-blue-200 {
		@apply border-blue-200;
	}

	include-color-blue-300 {
		@apply border-blue-300;
	}

	include-color-blue-400 {
		@apply border-blue-400;
	}

	include-color-blue-500 {
		@apply border-blue-500;
	}

	include-color-blue-600 {
		@apply border-blue-600;
	}
</style>
