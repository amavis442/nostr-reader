<script lang="ts">
	import CreateNoteModal from './partials/Modal/CreateNoteModal.svelte'
	import { openModal } from 'svelte-modals'
	import { publish } from '../lib/state/main'

	function createTextNote() {
		openModal(CreateNoteModal, {
			note: null,
			onSendTextNote: (noteText: string) => {
				publish(noteText, null)
			}
		})
	}
</script>

<div class="flex flex-col gap-4 h-screen">
	<div class="h-screen">
		<div
			id="Notes"
			class="flex flex-col relative mx-auto bg-gray-600
              dark:highlight-white/5 shadow-lg ring-1 ring-black/5
              divide-y ml-4 mr-4
              space-y-0 place-content-start
              h-full max-h-full w-11/12"
		>
			<div class="h-full w-full overflow-y-auto" id="realNotesContainer">
				<slot />
			</div>
		</div>
	</div>
</div>

<div class="createnote">
	<button on:click={createTextNote} class="create-note p-2 mr-4" title="Create a new note">
		<i class="fa-regular fa-message" />
	</button>
</div>

<style lang="postcss">
	div.createnote {
		position: absolute;
		bottom: 10px;
		right: 15%;
		border: 0;
	}
	.create-note {
		height: 50px;
		width: 50px;
	}
	.create-note i {
		height: 100%;
		font-size: 60px;
		color: rgba(255, 255, 255, 0.9);
	}
</style>
