import { Injectable } from '@angular/core';
import type { Mermaid } from 'mermaid';
import { MermaidThemeConverter } from './mermaid-theme.converter';

/**
 * The single lazy entry point for the mermaid library — the only runtime
 * `import('mermaid')` in the app (everything else uses `import type`), so a
 * page without a diagram never downloads it. `securityLevel: 'strict'` is
 * mermaid's own sanitizer (label HTML escaped, `click … call` disabled) and
 * `secure` locks the config keys a diagram source cannot override via
 * %%{init}%%.
 */
@Injectable({ providedIn: 'root' })
export class MermaidLoaderService {
    private loadPromise: Promise<Mermaid> | null = null;

    /** Loads mermaid once per session and initializes it with the app theme. */
    public load(): Promise<Mermaid> {
        this.loadPromise ??= import('mermaid')
            .then(({ default: mermaid }) => {
                mermaid.initialize({
                    startOnLoad: false,
                    securityLevel: 'strict',
                    theme: 'base',
                    suppressErrorRendering: true,
                    secure: [
                        'secure',
                        'securityLevel',
                        'startOnLoad',
                        'maxTextSize',
                        'suppressErrorRendering',
                        'maxEdges',
                        'themeCSS',
                        'themeVariables',
                        'fontFamily'
                    ],
                    themeVariables: MermaidThemeConverter.toThemeVariables(
                        token =>
                            getComputedStyle(document.documentElement)
                                .getPropertyValue(token)
                                .trim(),
                        getComputedStyle(document.body).fontFamily
                    )
                });
                return mermaid;
            })
            .catch((error: unknown) => {
                // A failed import is a transport error, not a permanent
                // condition — clear the memo so the next load() can retry.
                this.loadPromise = null;
                throw error;
            });
        return this.loadPromise;
    }
}
