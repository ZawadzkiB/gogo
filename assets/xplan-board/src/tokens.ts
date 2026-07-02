// Design tokens -- ported VERBATIM from xplan's web/src/styles/tokens.ts
// (the `TONE` + `FONTS` objects). Credit: xplan (ScreenTasks kanban board).
// The three STATE colors are the only saturated palette in the product;
// everything else stays muted so state reads first. src/board.css mirrors these
// same values as :root CSS custom properties -- change a value in both places.

export const TONE = {
  bg: '#141518',
  surface: '#1c1d21',
  surface2: '#23252a',
  surfaceHi: '#2b2d33',
  border: '#2a2c32',
  borderHi: '#363943',
  text: '#dcdde0',
  textDim: '#a4a6ab',
  muted: '#71747c',
  faint: '#494c54',

  // STATE colors -- the whole product hinges on these three.
  done: '#5db97a',                 // planned + in code
  doneFill: 'rgba(93,185,122,0.13)',
  doneStroke: 'rgba(93,185,122,0.65)',

  todoText: '#6c6f78',             // planned, not in code -- ghost
  todoStroke: '#3e424b',
  todoFill: 'rgba(110,114,124,0.06)',

  extra: '#e6a14a',                // in code, not in plan
  extraFill: 'rgba(230,161,74,0.13)',
  extraStroke: 'rgba(230,161,74,0.65)',

  accent: '#7aa8ff',               // selection
  accentFill: 'rgba(122,168,255,0.14)',
  danger: '#e0625c',
} as const;

export const FONTS = {
  ui: "'Inter', system-ui, -apple-system, sans-serif",
  mono: "'JetBrains Mono', 'IBM Plex Mono', ui-monospace, monospace",
  hand: "'Caveat', 'Bradley Hand', cursive",
} as const;
