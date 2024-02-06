import tailwindcss from 'tailwindcss';
import autoprefixer from 'autoprefixer';
import { fileURLToPath } from 'url';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const tailwindConfig = path.join(__dirname, 'tailwind.config.js');

export default {
  plugins: [
    //Some plugins, like tailwindcss/nesting, need to run before Tailwind,
    tailwindcss({ config: tailwindConfig }),
    //But others, like autoprefixer, need to run after,
    autoprefixer
  ]
};