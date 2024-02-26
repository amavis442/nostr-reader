'use strict'

/**
 * https://github.com/saideepesh000/svelte-read-more
 */ 
 
export const getMaxCharacters = (
	maxCharacters: number,
	isOpen: boolean,
	children: string,
	text: string
) => {
	if (maxCharacters) {
		if (isOpen) {
			text = children
		} else {
			text = children.substring(0, maxCharacters)
		}
		return text
	} else {
		return children
	}
}

export const isFullText = (truncatedText: string, text: string) => {
	return (
		truncatedText &&
		truncatedText.split('').filter((c) => c !== ' ').length ===
			text.split('').filter((c) => c !== ' ').length
	)
}

export const getMaxWords = (maxWords: number, isOpen: boolean, children: string, text: string) => {
	if (maxWords) {
		if (isOpen) {
			text = children
		} else {
			const words = children.split(' ').filter((c) => c !== '')
			text = words.slice(0, maxWords).join(' ')
		}
		return text
	} else {
		return children
	}
}

/** 
 * from https://www.npmjs.com/package/read-smore
 */

/**
 * Get Character Count
 */
export const getCharCount = (str: string): number => {
	return str.length
}

/**
 * Get Word Count
 */
export const getWordCount = (str: string):number  => {
	const words = removeTags(str).split(' ')
	return words.filter((word) => word.trim() !== '').length
}

/**
 * Trim whitespace
 */
export const trimSpaces = (str: string): string => {
	return str.replace(/(^\s*)|(\s*$)/gi, '')
}

/**
 * Remove HTML Tags from string
 */
export const removeTags = (str:string): string => {
	if (str === null || str === '') {
		return ''
	}

	return str.replace(/<[^>]+>/g, '')
}
