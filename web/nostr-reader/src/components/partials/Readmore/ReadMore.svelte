<script lang="ts">
	/**
	 * https://github.com/saideepesh000/svelte-read-more mixed with https://www.npmjs.com/package/read-smore and 
     * some small fixes to make it work with svelte
	 */
	import {
		getMaxCharacters,
		getMaxWords,
		isFullText,
		getWordCount,
		getCharCount,
		trimSpaces
	} from './utils'
	export let textContent: string
	export let readMoreLabel: string = 'Read more'
	export let readLessLabel: string = 'Read less'
	//export let maxChars: number
	export let maxWords: number
	export let dotDotDot: string = '...'

	let text
	let isOpen = false

	const ellipse = (str: string, max: number, isChars: boolean = false): string => {
		const trimmedSpaces = trimSpaces(str)

		if (isChars) {
			return trimmedSpaces.slice(0, max - 1)
		}

		const words = trimmedSpaces.split(/\s+/)
		return words.slice(0, max - 1).join(' ')
	}

	$: originalContentCount = getWordCount(textContent)
	$: truncateContent = ellipse(textContent, maxWords, false)

	$: cleanText = textContent.replace(/\s+/g, ' ').trim()

	$: finalLabel = isOpen ? readLessLabel : readMoreLabel
	//$: maxCharsText = getMaxCharacters(maxChars, isOpen, textContent, text)
	$: finalText = isOpen ? cleanText : truncateContent //getMaxWords(maxWords, isOpen, maxCharsText, text)
	$: finalSymbol = isOpen ? '' : dotDotDot
	$: showButton = !isOpen && isFullText(finalText, cleanText) ? false : true

	const handleClick = () => {
		isOpen = !isOpen
	}
</script>

<div data-testid="wrapper">
	{@html finalText}


	<span data-testid="button-wrapper" data-visible={`${showButton}`} class="button-wrapper">
		{!isOpen ? finalSymbol : ''}
        <br/>
		<button data-testid="button" on:click={handleClick} class="button">
			{finalLabel}
		</button>
	</span>
</div>

<style>
	/* custom styles */
	.button-wrapper {
		margin-top: 1em;
        display: block;
	}
	span[data-visible='false'] {
		visibility: hidden;
	}
	.button {
		border: 0;
		background-color: transparent;
		text-decoration: underline;
		cursor: pointer;
	}
	.button::first-letter {
		text-transform: uppercase;
	}
	.button:hover {
		text-decoration: none;
	}
</style>
