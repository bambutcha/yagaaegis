/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: {
          DEFAULT: '#12140f',
          soft: '#2a2f24',
          mute: '#5c6354',
        },
        paper: {
          DEFAULT: '#e8e4d9',
          deep: '#d9d3c4',
          mist: '#f3f0e8',
        },
        signal: {
          DEFAULT: '#1f6b4a',
          bright: '#2d9a68',
          warn: '#b45309',
          danger: '#b42318',
        },
      },
      fontFamily: {
        display: ['Syne', 'system-ui', 'sans-serif'],
        sans: ['IBM Plex Sans', 'system-ui', 'sans-serif'],
        mono: ['IBM Plex Mono', 'ui-monospace', 'monospace'],
      },
      boxShadow: {
        stamp: '0 0 0 1px rgba(18, 20, 15, 0.12)',
      },
      keyframes: {
        rise: {
          '0%': {opacity: '0', transform: 'translateY(10px)'},
          '100%': {opacity: '1', transform: 'translateY(0)'},
        },
        pulseDot: {
          '0%, 100%': {opacity: '1'},
          '50%': {opacity: '0.35'},
        },
        scanline: {
          '0%': {transform: 'translateY(-100%)'},
          '100%': {transform: 'translateY(100%)'},
        },
      },
      animation: {
        rise: 'rise 0.45s ease-out both',
        'pulse-dot': 'pulseDot 1.6s ease-in-out infinite',
        scanline: 'scanline 1.8s linear infinite',
      },
    },
  },
  plugins: [],
};
