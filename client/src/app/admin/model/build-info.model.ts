/** Release identity of the running backend, stamped at build time. */
export interface BuildInfo {
    version: string;
    commit: string;
}
