/* used by ./tailwind to preprocess styles into tailwind CSS
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./views/**/*html"],
  theme: {
    extend: {},
  },
  plugins: [require('@tailwindcss/forms'),],
  mode: 'jit',
}

