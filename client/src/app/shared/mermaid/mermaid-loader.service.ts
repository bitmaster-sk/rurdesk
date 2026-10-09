import { Injectable } from '@angular/core';
import type { Mermaid } from 'mermaid';
import { MermaidThemeConverter } from './mermaid-theme.converter';

/**
 * The single lazy entry point for the mermaid library. The dynamic import (and
 * the one-time `initialize`) run on the first `load()` and never again — no
 * other file imports `mermaid` at runtime (only `import type`), and nothing is
 * added to `angular.json` scripts, so a page without a diagram never pays for
 * the download. `securityLevel: 'strict'` is mermaid's own sanitizer: label
 * HTML is escaped and `click … call` callbacks are disabled, so no second
 * sanitizer (DOMPurify etc.) is layered on top.
 */
@Injectable({ providedIn: 'root' })
export class MermaidLoaderService {
    private loadPromise: Promise<Mermaid> | null = null;

    /** Loads mermaid once per session and initializes it with the app theme. */
    public load(): Promise<Mermaid> {
        this.loadPromise ??= import('mermaid').then(({ default: mermaid }) => {
            mermaid.initialize({
                startOnLoad: false,
                securityLevel: 'strict',
                theme: 'base',
                suppressErrorRendering: true,
                themeVariables: MermaidThemeConverter.toThemeVariables(
                    token =>
                        getComputedStyle(document.documentElement).getPropertyValue(token).trim(),
                    getComputedStyle(document.body).fontFamily
                )
            });
            return mermaid;
        });
        return this.loadPromise;
    }
}
