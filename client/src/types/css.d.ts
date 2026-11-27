declare module '*.css';
/* A browser spec that applies a stylesheet to one test only takes its text. */
declare module '*.css?inline' {
    const css: string;
    export default css;
}
