<script lang="ts">
	import { onMount } from 'svelte'
	import Button from '../partials/Button.svelte'
	import Text from '../partials/Text.svelte'
	import { addToast } from '../partials/Toast/toast'

	let name: string
	let about: string
	let picture: string
	let nip05: string
	let website: string
	let displayname: string
	let lud16: string

	/**
	 * @see https://www.thisdot.co/blog/handling-forms-in-svelte
	 * @param e
	 */
	function onSubmit() {
		if (name) {
			name = name.trim()
			if (name && !name.match(/^\w[\w\-]+\w$/i)) {
				addToast({
					message:
						'Account name not correct! George-Washington-1776 is a valid <username>, but George Washington is not',
					type: 'error',
					dismissible: true,
					timeout: 3000
				})
				return
			}
		}

		fetch(`${import.meta.env.VITE_API_LINK}/api/setmetadata`, {
			method: 'POST',
			body: JSON.stringify({
				name: name,
				about: about,
				picture: picture,
				nip05: nip05,
				website: website,
				display_name: displayname,
				lud16: lud16
			}),
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((response) => {
				console.log('Json is ', response)
				return response
			})
			.catch((err) => {
				console.error('error', err)
			})

		addToast({
			message: 'Account updated!',
			type: 'success',
			dismissible: true,
			timeout: 3000
		})
	}

	function getMetaData() {
		fetch(`${import.meta.env.VITE_API_LINK}/api/getmetadata`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((reponse) => {
				console.log('Json is ', reponse)
				let profile = JSON.parse(reponse.data.content)
				name = profile.name
				about = profile.about
				picture = profile.picture
				nip05 = profile.nip05
				displayname = profile.display_name
				website = profile.website
				lud16 = profile.lud16
				return reponse
			})
			.catch((err) => {
				console.error('error', err)
			})
	}

	onMount(async () => {
		fetch(`${import.meta.env.VITE_API_LINK}/api/getprofile`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json'
			}
		})
			.then((res) => {
				return res.json()
			})
			.then((response) => {
				console.log('Json is ', response)
				const profile = response.data
				name = profile.name
				about = profile.about
				picture = profile.picture
				nip05 = profile.nip05
				displayname = profile.display_name
				website = profile.website
				lud16 = profile.lud16
				return profile
			})
			.catch((err) => {
				console.error('error', err)
			})
	})
</script>

<div class="w-10/12  items-center p-2">
	<div class="block p-6 rounded-lg shadow-lg bg-white w-full ml-6 mt-6 ">
		<form on:submit|preventDefault={onSubmit}>
			<div class="form-group mb-6">
				<div class="md:w-2/12 flex justify-end">
					<Button click={getMetaData}>Metadata</Button>
				</div>
			</div>

			<div class="form-group mb-6">
				<label for="myname" class="form-label inline-block mb-2 text-gray-700"> Name </label>
				<Text bind:value={name} id="myname" describedby="nameHelp" placeholder="Name" />
				<small id="nameHelp" class="block mt-1 text-xs text-gray-600">
					Name to be used instead of your public key
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="aboutme" class="form-label inline-block mb-2 text-gray-700"> About </label>
				<Text bind:value={about} id="aboutme" describedby="aboutHelp" placeholder="About" />
				<small id="aboutHelp" class="block mt-1 text-xs text-gray-600">
					Tell us something about you. Any hobby's, what do you like/dislike?
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="nip05" class="form-label inline-block mb-2 text-gray-700"> Nip05 </label>
				<Text bind:value={nip05} id="nip05" describedby="nip05Help" placeholder="Nip05" />
				<small id="nip05Help" class="block mt-1 text-xs text-gray-600">
					Some relays require a nip05 verification before you can post in the form of
					bob@example.com
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="lud16" class="form-label inline-block mb-2 text-gray-700"> Lud16 </label>
				<Text bind:value={lud16} id="lud16" describedby="lud16Help" placeholder="Lud16" />
				<small id="lud16Help" class="block mt-1 text-xs text-gray-600">
					If you want to receive payments like sats, you can use https://getalby.com/ and lud16 will
					be something like name@getalby.com
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="website" class="form-label inline-block mb-2 text-gray-700"> Website </label>
				<Text bind:value={website} id="website" describedby="websiteHelp" placeholder="Website" />
				<small id="websiteHelp" class="block mt-1 text-xs text-gray-600">
					What website can you be found
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="displayname" class="form-label inline-block mb-2 text-gray-700">
					Display Name
				</label>
				<Text
					bind:value={displayname}
					id="displayname"
					describedby="displaynameHelp"
					placeholder="Display Name"
				/>
				<small id="displaynameHelp" class="block mt-1 text-xs text-gray-600">
					Different name
				</small>
			</div>

			<div class="form-group mb-6">
				<label for="pictureofme" class="form-label inline-block mb-2 text-gray-700">
					Picture
				</label>
				<Text
					bind:value={picture}
					id="pictureofme"
					describedby="pictureHelp"
					placeholder="Picture url"
				/>
				<small id="pictureHelp" class="block mt-1 text-xs text-gray-600">
					A nice avatar or profile picture. This is a link to an external file somewhere on the net.
					Pictures are not stored in relays.
				</small>
			</div>
			<div class="flex justify-end w-full gap-2">
				<div class="col-2">
					<Button type="submit">Submit</Button>
				</div>
			</div>
		</form>
	</div>
</div>
