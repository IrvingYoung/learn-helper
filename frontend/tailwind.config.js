/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        display: ['var(--font-display)', 'serif'],
        body: ['var(--font-body)', 'sans-serif'],
        mono: ['var(--font-mono)', 'monospace'],
      },
      colors: {
        'th-bg-primary': 'var(--bg-primary)',
        'th-bg-secondary': 'var(--bg-secondary)',
        'th-bg-tertiary': 'var(--bg-tertiary)',
        'th-border': 'var(--border)',
        'th-border-hover': 'var(--border-hover)',
        'th-text-primary': 'var(--text-primary)',
        'th-text-secondary': 'var(--text-secondary)',
        'th-text-muted': 'var(--text-muted)',
        'th-accent': 'var(--accent)',
        'th-accent-light': 'var(--accent-light)',
        'th-accent-bg': 'var(--accent-bg)',
        'th-success': 'var(--success)',
        'th-warning': 'var(--warning)',
        'th-error': 'var(--error)',
        'th-node-filled': 'var(--node-filled)',
        'th-node-partial': 'var(--node-partial)',
        'th-node-empty': 'var(--node-empty)',
        'th-input-bg': 'var(--input-bg)',
        'th-input-border': 'var(--input-border)',
        'th-user-bubble': 'var(--user-bubble-bg)',
        'th-user-bubble-text': 'var(--user-bubble-text)',
        'th-assistant-bubble': 'var(--assistant-bubble-bg)',
        'th-assistant-bubble-text': 'var(--assistant-bubble-text)',
      },
      boxShadow: {
        'th': 'var(--shadow)',
      },
    },
  },
  plugins: [],
}