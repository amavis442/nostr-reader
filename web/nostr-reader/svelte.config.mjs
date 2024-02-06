import { vitePreprocess } from '@sveltejs/vite-plugin-svelte'
import sveltePreprocess  from 'svelte-preprocess'
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const postcssConfig = path.join(__dirname, 'postcss.config.js');

export default {
  preprocess: [
      // Consult https://svelte.dev/docs#compile-time-svelte-preprocess
    // for more information about preprocessors
    vitePreprocess(),
    sveltePreprocess({
      postcss: {
        configFilePath: postcssConfig
      }
    })
  ]
};