// Contenteditable editor engine: DOM→string serialization. Serialization and the
// caret↔offset mapping in EditorSelection share ONE ordered atom list (buildAtoms)
// so their length rules can never diverge.
// Empirically verified across Chromium/Firefox/WebKit.

import { EditorChip } from './editor-chip';

export interface Atom {
    node: Node;
    kind: 'text' | 'chip' | 'br' | 'blocknl';
    str: string;
    base: number;
}

export abstract class EditorText {
    // SINGLE source of truth for length rules — consumed by both serialize() and
    // EditorSelection's caret↔offset functions.
    public static buildAtoms(root: HTMLElement): Atom[] {
        const atoms: Atom[] = [];
        const endsNL = (): boolean => {
            const l = atoms[atoms.length - 1];
            return !l || l.str.endsWith('\n');
        };

        function rec(node: Node): void {
            node.childNodes.forEach(c => {
                if (c.nodeType === 3) {
                    const data = (c as Text).data;
                    if (data.length) {
                        atoms.push({ node: c, kind: 'text', str: data, base: 0 });
                    }
                } else if (EditorChip.isChip(c)) {
                    atoms.push({
                        node: c,
                        kind: 'chip',
                        str: c.dataset['token'] ?? '',
                        base: 0
                    });
                } else if (c.nodeType === 1 && (c as HTMLElement).tagName === 'BR') {
                    atoms.push({ node: c, kind: 'br', str: '\n', base: 0 });
                } else if (c.nodeType === 1) {
                    if (EditorText.isBlockEl(c) && atoms.length && !endsNL()) {
                        atoms.push({ node: c, kind: 'blocknl', str: '\n', base: 0 });
                    }
                    rec(c);
                }
            });
        }

        rec(root);
        let base = 0;
        for (const a of atoms) {
            a.base = base;
            base += a.str.length;
        }
        return atoms;
    }

    // DOM → string. Trailing newlines (browser's trailing <div><br></div>) are trimmed.
    public static serialize(root: HTMLElement): string {
        return EditorText.buildAtoms(root)
            .map(a => a.str)
            .join('')
            .replace(/\n+$/, '');
    }

    // DOM → string WITHOUT trimming trailing newlines. Shares the exact coordinate
    // space of EditorSelection (both go through buildAtoms), so line-offset math
    // against a caret from getLinearSelection stays correct even when the caret sits
    // on an empty trailing line that serialize() would drop.
    public static serializeRaw(root: HTMLElement): string {
        return EditorText.buildAtoms(root)
            .map(a => a.str)
            .join('');
    }

    private static isBlockEl(n: Node): boolean {
        if (n.nodeType !== 1) return false;
        const el = n as HTMLElement;
        if (el.tagName === 'BR') return false;
        const d = getComputedStyle(el).display;
        return (
            d === 'block' ||
            d === 'list-item' ||
            ['DIV', 'P', 'LI', 'UL', 'OL', 'PRE', 'BLOCKQUOTE', 'H1', 'H2', 'H3'].includes(
                el.tagName
            )
        );
    }
}
