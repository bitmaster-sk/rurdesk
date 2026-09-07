// Linear caret-offset ↔ DOM position mapping. Every offset here is an index into
// EditorText.serializeRaw(root) — both sides walk the same atom list.

import { Atom, EditorText } from './editor-text';

export abstract class EditorSelection {
    // Current DOM selection → linear offsets in serialize(root).
    public static getLinearSelection(root: HTMLElement): { start: number; end: number } {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount) return { start: 0, end: 0 };
        const r = sel.getRangeAt(0);
        return {
            start: EditorSelection.posToLinear(root, r.startContainer, r.startOffset),
            end: EditorSelection.posToLinear(root, r.endContainer, r.endOffset)
        };
    }

    // Place the DOM selection at linear offsets in serialize(root).
    public static setLinearSelection(root: HTMLElement, start: number, end: number): void {
        const doc = root.ownerDocument;
        const s = EditorSelection.locate(root, start);
        const e = EditorSelection.locate(root, end);
        const r = doc.createRange();
        r.setStart(s.node, s.offset);
        r.setEnd(e.node, e.offset);
        const sel = doc.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(r);
    }

    private static atomsWithin(atoms: Atom[], node: Node): Atom[] {
        return atoms.filter(a => a.node === node || node.contains(a.node));
    }

    private static startLinearOfNode(atoms: Atom[], node: Node): number {
        const w = EditorSelection.atomsWithin(atoms, node);
        if (w.length) return w[0].base;
        let acc = 0;
        for (const a of atoms) {
            if (node.compareDocumentPosition(a.node) & Node.DOCUMENT_POSITION_PRECEDING) {
                acc = a.base + a.str.length;
            }
        }
        return acc;
    }

    private static endLinearOfNode(atoms: Atom[], node: Node): number {
        const w = EditorSelection.atomsWithin(atoms, node);
        if (w.length) {
            const l = w[w.length - 1];
            return l.base + l.str.length;
        }
        return EditorSelection.startLinearOfNode(atoms, node);
    }

    private static posToLinear(root: HTMLElement, container: Node, offset: number): number {
        const atoms = EditorText.buildAtoms(root);
        if (container.nodeType === 3) {
            const a = atoms.find(x => x.node === container);
            if (a) return a.base + offset;
            return EditorSelection.startLinearOfNode(atoms, container);
        }
        const kids = container.childNodes;
        if (offset >= kids.length) return EditorSelection.endLinearOfNode(atoms, container);
        return EditorSelection.startLinearOfNode(atoms, kids[offset]);
    }

    private static locate(root: HTMLElement, target: number): { node: Node; offset: number } {
        const atoms = EditorText.buildAtoms(root);
        for (const a of atoms) {
            const end = a.base + a.str.length;
            if (target < end || (target === end && a.kind === 'text')) {
                if (a.kind === 'text') return { node: a.node, offset: target - a.base };
                const parent = a.node.parentNode!;
                const idx = Array.prototype.indexOf.call(parent.childNodes, a.node);
                return { node: parent, offset: target - a.base === 0 ? idx : idx + 1 };
            }
        }
        if (atoms.length) {
            const a = atoms[atoms.length - 1];
            if (a.kind === 'text') return { node: a.node, offset: (a.node as Text).data.length };
            const parent = a.node.parentNode!;
            const idx = Array.prototype.indexOf.call(parent.childNodes, a.node);
            return { node: parent, offset: idx + 1 };
        }
        return { node: root, offset: 0 };
    }
}
