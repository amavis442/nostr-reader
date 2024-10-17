import defaultTheme from 'tailwindcss/defaultTheme';
import colors from 'tailwindcss/colors'

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{html,js,svelte,ts}'
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter var', ...defaultTheme.fontFamily.sans],
      },
      height: {
        '5p': '5%',
        '10p': '10%',
        '20p': '20%',
        '60p': '60%',
        '80p': '80%',
        '85p': '85%',
        '90p': '90%',
        '100p': '100%'
      }
    },
    gridTemplateColumns:
    {
      '20/80': '20% 80%',
      '20/40/20': '20% 60% 20%',
      'fixed': '40px 260px',
    }, 
  },
  plugins: [],
}

