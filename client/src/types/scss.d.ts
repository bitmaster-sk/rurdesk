/* Browser specs side-effect-import component stylesheets so the CSS under test
   is actually loaded by Vite; TypeScript 6.0 errors on side-effect imports it
   cannot resolve (TS2882), so the wildcard needs a declaration. */
declare module '*.scss';
