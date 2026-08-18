// Contenteditable editor engine: DOM↔string serialization + linear caret-offset
// mapping. Both serialize() and the caret↔offset functions share ONE ordered
// atom list (buildAtoms) so their length rules can never diverge.
// Empirically verified across Chromium/Firefox/WebKit.

export interface Atom {
    node: Node;
    kind: 'text' | 'chip' | 'br' | 'blocknl';
    str: string;
    base: number;
}

export function isChip(n: Node): n is HTMLElement {
    return n.nodeType === 1 && (n as HTMLElement).classList?.contains('mention-chip');
}

function isBlockEl(n: Node): boolean {
    if (n.nodeType !== 1) return false;
    const el = n as HTMLElement;
    if (el.tagName === 'BR') return false;
    const d = getComputedStyle(el).display;
    return (
        d === 'block' ||
        d === 'list-item' ||
        ['DIV', 'P', 'LI', 'UL', 'OL', 'PRE', 'BLOCKQUOTE', 'H1', 'H2', 'H3'].includes(el.tagName)
    );
}

// SINGLE source of truth for length rules — consumed by both serialize() and
// the caret↔offset functions below.
export function buildAtoms(root: HTMLElement): Atom[] {
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
            } else if (isChip(c)) {
                atoms.push({
                    node: c,
                    kind: 'chip',
                    str: c.dataset['token'] ?? '',
                    base: 0
                });
            } else if (c.nodeType === 1 && (c as HTMLElement).tagName === 'BR') {
                atoms.push({ node: c, kind: 'br', str: '\n', base: 0 });
            } else if (c.nodeType === 1) {
                if (isBlockEl(c) && atoms.length && !endsNL()) {
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
export function serialize(root: HTMLElement): string {
    return buildAtoms(root)
        .map(a => a.str)
        .join('')
        .replace(/\n+$/, '');
}

// DOM → string WITHOUT trimming trailing newlines. Shares the exact coordinate
// space of getLinearSelection/setLinearSelection (both go through buildAtoms),
// so line-offset math against a caret from getLinearSelection stays correct even
// when the caret sits on an empty trailing line that serialize() would drop.
export function serializeRaw(root: HTMLElement): string {
    return buildAtoms(root)
        .map(a => a.str)
        .join('');
}

function atomsWithin(atoms: Atom[], node: Node): Atom[] {
    return atoms.filter(a => a.node === node || node.contains(a.node));
}

function startLinearOfNode(atoms: Atom[], node: Node): number {
    const w = atomsWithin(atoms, node);
    if (w.length) return w[0].base;
    let acc = 0;
    for (const a of atoms) {
        if (node.compareDocumentPosition(a.node) & Node.DOCUMENT_POSITION_PRECEDING) {
            acc = a.base + a.str.length;
        }
    }
    return acc;
}

function endLinearOfNode(atoms: Atom[], node: Node): number {
    const w = atomsWithin(atoms, node);
    if (w.length) {
        const l = w[w.length - 1];
        return l.base + l.str.length;
    }
    return startLinearOfNode(atoms, node);
}

function posToLinear(root: HTMLElement, container: Node, offset: number): number {
    const atoms = buildAtoms(root);
    if (container.nodeType === 3) {
        const a = atoms.find(x => x.node === container);
        if (a) return a.base + offset;
        return startLinearOfNode(atoms, container);
    }
    const kids = container.childNodes;
    if (offset >= kids.length) return endLinearOfNode(atoms, container);
    return startLinearOfNode(atoms, kids[offset]);
}

// Current DOM selection → linear offsets in serialize(root).
export function getLinearSelection(root: HTMLElement): { start: number; end: number } {
    const sel = root.ownerDocument.getSelection();
    if (!sel?.rangeCount) return { start: 0, end: 0 };
    const r = sel.getRangeAt(0);
    return {
        start: posToLinear(root, r.startContainer, r.startOffset),
        end: posToLinear(root, r.endContainer, r.endOffset)
    };
}

function locate(root: HTMLElement, target: number): { node: Node; offset: number } {
    const atoms = buildAtoms(root);
    for (const a of atoms) {
        const end = a.base + a.str.length;
        if (target < end || (target === end && a.kind === 'text')) {
            if (a.kind === 'text') return { node: a.node, offset: target - a.base };
            const parent = a.node.parentNode!;
            const idx = Array.prototype.indexOf.call(parent.childNodes, a.node) as number;
            return { node: parent, offset: target - a.base === 0 ? idx : idx + 1 };
        }
    }
    if (atoms.length) {
        const a = atoms[atoms.length - 1];
        if (a.kind === 'text') return { node: a.node, offset: (a.node as Text).data.length };
        const parent = a.node.parentNode!;
        const idx = Array.prototype.indexOf.call(parent.childNodes, a.node) as number;
        return { node: parent, offset: idx + 1 };
    }
    return { node: root, offset: 0 };
}

// Place the DOM selection at linear offsets in serialize(root).
export function setLinearSelection(root: HTMLElement, start: number, end: number): void {
    const doc = root.ownerDocument;
    const s = locate(root, start);
    const e = locate(root, end);
    const r = doc.createRange();
    r.setStart(s.node, s.offset);
    r.setEnd(e.node, e.offset);
    const sel = doc.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(r);
}

// Build an atomic chip element for insertion.
export function buildChip(idUser: number, name: string): HTMLElement {
    const s = document.createElement('span');
    s.className = 'mention-chip';
    s.contentEditable = 'false';
    s.dataset['token'] = `@[${name}](user:${idUser})`;
    s.dataset['id'] = String(idUser);
    s.textContent = '@' + name;
    return s;
}

// True when the caret is at a valid '@' mention opener:
// preceding char is whitespace/BOF, or the immediately preceding DOM node is a chip.
export function atMentionBoundary(root: HTMLElement): boolean {
    const sel = root.ownerDocument.getSelection();
    if (!sel?.rangeCount) return false;
    const r = sel.getRangeAt(0);
    const c = r.startContainer;
    const o = r.startOffset;
    if (c.nodeType === 3) {
        if (o === 0) {
            const prev = c.previousSibling;
            return !prev || isChip(prev);
        }
        return /\s/.test((c as Text).data[o - 1]);
    }
    const prev = c.childNodes[o - 1];
    return !prev || isChip(prev) || (prev.nodeType === 3 && /\s$/.test((prev as Text).data));
}

// --- Programmatic edits: execCommand for undoable text; Range for chip node ---

// Sanitize paste to plain text (undoable via execCommand).
// Returns a cleanup function that removes the listener.
export function installPasteSanitizer(root: HTMLElement): () => void {
    const handler = (e: ClipboardEvent): void => {
        e.preventDefault();
        const text = e.clipboardData?.getData('text/plain') ?? '';
        root.ownerDocument.execCommand('insertText', false, text);
    };
    root.addEventListener('paste', handler);
    return (): void => {
        root.removeEventListener('paste', handler);
    };
}

// Insert markdown markers around the current selection (undoable).
// With a collapsed selection, the caret is repositioned between the markers
// so it sits at `before|after` rather than `beforeafter|`.
export function applyMarker(root: HTMLElement, before: string, after: string): void {
    const sel = root.ownerDocument.getSelection();
    const collapsed = !sel?.rangeCount || sel.getRangeAt(0).collapsed;
    const text = sel?.rangeCount ? sel.getRangeAt(0).toString() : '';
    root.ownerDocument.execCommand('insertText', false, before + text + after);
    if (collapsed && after.length > 0) {
        // Move caret back by after.length so it sits between the markers.
        const { start } = getLinearSelection(root);
        const target = start - after.length;
        setLinearSelection(root, target, target);
    }
}

// Insert a chip at the caret position, then place the caret after the trailing space.
export function insertChipAtCaret(root: HTMLElement, idUser: number, name: string): void {
    const sel = root.ownerDocument.getSelection();
    if (!sel?.rangeCount) return;
    const r = sel.getRangeAt(0);
    r.deleteContents();
    const chip = buildChip(idUser, name);
    const space = root.ownerDocument.createTextNode(' ');
    r.insertNode(space);
    r.insertNode(chip);
    const nr = root.ownerDocument.createRange();
    nr.setStartAfter(space);
    nr.collapse(true);
    sel.removeAllRanges();
    sel.addRange(nr);
}
