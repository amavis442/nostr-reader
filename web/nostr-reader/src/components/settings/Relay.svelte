<script lang="ts">
	import { onMount } from 'svelte'
	import { addToast } from '../partials/Toast/toast'
	import { writable } from 'svelte/store'

	let url: string
	let write: boolean = true
	let read: boolean = true
	let search: boolean = true

	type Relay = {
		ID: Number
		url: string
		read: boolean
		write: boolean
		search: boolean
		CreatedAt: Date | null | undefined
		UpdatedAt: Date | null | undefined
	}

	const relays = writable<Array<Relay>>([])

	/**
	 * @see https://www.thisdot.co/blog/handling-forms-in-svelte
	 * @param e
	 */
	function addRelay() {
		fetch(`${import.meta.env.VITE_API_LINK}/api/addrelay`, {
			method: 'POST',
			body: JSON.stringify({
				url: url,
				write: new Boolean(write),
				read: new Boolean(read),
				search: new Boolean(search)
			}),
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((data) => {
				console.log('Json is ', data)

				if (data.result.status == 'ok') {
					addToast({
						message: 'Relay added!',
						type: 'success',
						dismissible: true,
						timeout: 3000
					})

					relays.set(data.relays ? data.relays : [])
				}
				if (data.result.status == 'error') {
					addToast({
						message: 'Relay could not be added: ' + data.result.msg,
						type: 'error',
						dismissible: true,
						timeout: 3000
					})
				}

				return data
			})
			.catch((err) => {
				console.error('error', err)
			})
	}

	function removeRelay(url: string) {
		fetch(`${import.meta.env.VITE_API_LINK}/api/removerelay`, {
			method: 'POST',
			body: JSON.stringify({
				url: url
			}),
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((data) => {
				if (data.result.status == 'ok') {
					addToast({
						message: 'Relay removed!',
						type: 'success',
						dismissible: true,
						timeout: 3000
					})
					relays.set(data.relays ? data.relays : [])
				}
				if (data.result.status == 'error') {
					addToast({
						message: 'Relay could not be removed: ' + data.result.msg,
						type: 'error',
						dismissible: true,
						timeout: 3000
					})
				}

				return data
			})
			.catch((err) => {
				console.error('error', err)
			})

		return null
	}

	onMount(async () => {
		fetch(`${import.meta.env.VITE_API_LINK}/api/getrelays`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((data) => {
				console.log('Json is ', data)
				console.debug('Relay data ', data.relays)
				relays.set(data.relays ? data.relays : [])
				return data
			})
			.catch((err) => {
				console.error('error', err)
			})
	})
</script>

<div class="xl:w-8/12 lg:w-10/12 md:w-10/12 sm:w-full">
	<div class="block p-6 rounded-lg shadow-lg w-full ml-6 mt-6 bg-blue-200">
		<form on:submit|preventDefault>
			<div class="row">
				<div class="flex justify-end w-full gap-2">
					<div class="justify-items-start w-7/12">
						<label for="myname" class="text-gray-700 w-1/12">Url </label>
						<input
							type="text"
							class="text"
							bind:value={url}
							id="relay-url"
							aria-describedby="relayUrl"
							placeholder="wss://<name of relay>"
						/>
					</div>
					<div class="w-5/12 flex justify-end">
						<span
							><input type="checkbox" bind:checked={write} id="relay-write" />
							<label for="relay-write" class="p-1">Write</label></span
						>
						<span
							><input type="checkbox" bind:checked={read} id="relay-read" />
							<label for="relay-read" class="p-1">Read</label></span
						>
						<span
							><input type="checkbox" bind:checked={search} id="relay-search" />
							<label for="relay-search" class="p-1">Search</label></span
						>
					</div>
				</div>

				<div class="flex justify-end w-full gap-2">
					<div class="col-2">
						<button type="button" on:click={addRelay} class="btn">
							<i class="fa-solid fa-circle-plus"></i> Add
						</button>
					</div>
				</div>

				<hr class="m-2" />

				{#each $relays as relay (relay.url)}
					<div class="flex space-x-1 p-2">
						<div class="justify-items-start w-6/12">
							<strong>{relay.url}</strong>
						</div>
						<div class="justify-items-center w-3/12 p-1">
							{#if relay.write}<i class="fa-solid fa-pen"></i>{/if}
							{#if relay.read}<i class="fa-solid fa-book-open"></i>{/if}
							{#if relay.search}<i class="fa-solid fa-magnifying-glass"></i>{/if}
						</div>
						<div class="w-3/12 flex justify-end">
							<button type="button" on:click={removeRelay(relay.url)} class="btn-remove">
								<i class="fa-regular fa-circle-xmark"></i> Delete
							</button>
						</div>
					</div>
				{/each}
			</div>
		</form>
	</div>
</div>

<style lang="postcss">
	.text {
		@apply w-11/12 px-3 py-1.5 text-base font-normal
        text-gray-700 bg-white bg-clip-padding border border-solid
        border-gray-300 rounded transition ease-in-out m-0 
		focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none;
	}
	.btn {
		@apply px-6 py-2.5 bg-blue-600 text-white font-medium text-xs
          leading-tight uppercase rounded shadow-md hover:bg-blue-700
          hover:shadow-lg focus:bg-blue-700 focus:shadow-lg focus:outline-none
          focus:ring-0 active:bg-blue-800 active:shadow-lg transition
          duration-150 ease-in-out;
	}

	.btn-remove {
		@apply px-6 py-2.5 bg-red-600 text-white font-medium text-xs
          leading-tight uppercase rounded shadow-md hover:bg-red-700
          hover:shadow-lg focus:bg-red-700 focus:shadow-lg focus:outline-none
          focus:ring-0 active:bg-red-800 active:shadow-lg transition
          duration-150 ease-in-out;
	}
</style>
