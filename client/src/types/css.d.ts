/* Same as scss.d.ts but for plain CSS side-effect imports in browser specs
   (Tailwind entry + flatpickr base stylesheet), which Vite loads but
   TypeScript 6.0 cannot resolve (TS2882) without a wildcard declaration. */
declare module '*.css';
