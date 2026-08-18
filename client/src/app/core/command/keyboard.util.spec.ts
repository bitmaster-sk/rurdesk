import { describe, it, expect } from 'vitest';
import { isEditableTarget, isInOverlay, resolveHotkey } from './keyboard.util';

const key = (init: Partial<KeyboardEvent> & { key: string }): KeyboardEvent =>
    ({
        metaKey: false,
        ctrlKey: false,
        isComposing: false,
        target: null,
        ...init
    }) as unknown as KeyboardEvent;

describe('isEditableTarget', () => {
    it('true for input/textarea/select/contenteditable', () => {
        expect(isEditableTarget({ tagName: 'INPUT' } as unknown as EventTarget)).toBe(true);
        expect(
            isEditableTarget({ tagName: 'DIV', isContentEditable: true } as unknown as EventTarget)
        ).toBe(true);
    });
    it('false for plain div and null', () => {
        expect(
            isEditableTarget({ tagName: 'DIV', isContentEditable: false } as unknown as EventTarget)
        ).toBe(false);
        expect(isEditableTarget(null)).toBe(false);
    });
});

describe('isInOverlay', () => {
    it('true only when an ancestor is the CDK overlay container', () => {
        const inside = {
            closest: (sel: string) => (sel === '.cdk-overlay-container' ? {} : null)
        } as unknown as EventTarget;
        expect(isInOverlay(inside)).toBe(true);
        expect(isInOverlay({ closest: () => null } as unknown as EventTarget)).toBe(false);
        expect(isInOverlay(null)).toBe(false);
    });
});

describe('resolveHotkey', () => {
    const ctx = (paletteOpen = false, helpOpen = false) => ({ paletteOpen, helpOpen });
    it('Cmd+K opens (all) / closes when open', () => {
        expect(resolveHotkey(key({ key: 'k', metaKey: true }), ctx(false))).toEqual({
            type: 'open',
            mode: 'all'
        });
        expect(resolveHotkey(key({ key: 'k', metaKey: true }), ctx(true))).toEqual({
            type: 'close'
        });
    });
    it('ignores everything else while the palette is open', () => {
        expect(resolveHotkey(key({ key: '/' }), ctx(true))).toEqual({ type: 'none' });
    });
    it('ignores bare keys while the help sheet is open', () => {
        expect(resolveHotkey(key({ key: '/' }), ctx(false, true))).toEqual({ type: 'none' });
        expect(resolveHotkey(key({ key: '?' }), ctx(false, true))).toEqual({ type: 'none' });
        expect(resolveHotkey(key({ key: 'j' }), ctx(false, true))).toEqual({ type: 'none' });
    });
    it('never fires bare keys in an editable target or during IME', () => {
        expect(
            resolveHotkey(
                key({ key: '/', target: { tagName: 'INPUT' } as unknown as EventTarget }),
                ctx(false)
            )
        ).toEqual({ type: 'none' });
        expect(resolveHotkey(key({ key: '/', isComposing: true }), ctx(false))).toEqual({
            type: 'none'
        });
    });
    // Regression: arrows inside an open dropdown moved both its highlight and the
    // task row behind it.
    it('never fires bare keys from inside a CDK overlay', () => {
        const inOverlay = {
            tagName: 'BUTTON',
            closest: (sel: string) => (sel === '.cdk-overlay-container' ? {} : null)
        } as unknown as EventTarget;
        expect(resolveHotkey(key({ key: 'ArrowDown', target: inOverlay }), ctx(false))).toEqual({
            type: 'none'
        });
        expect(resolveHotkey(key({ key: '/', target: inOverlay }), ctx(false))).toEqual({
            type: 'none'
        });
    });
    it('bare / opens navigation, ? opens help, j/k & arrows move the list', () => {
        expect(resolveHotkey(key({ key: '/' }), ctx(false))).toEqual({
            type: 'open',
            mode: 'navigation'
        });
        expect(resolveHotkey(key({ key: '?' }), ctx(false))).toEqual({ type: 'help' });
        expect(resolveHotkey(key({ key: 'j' }), ctx(false))).toEqual({
            type: 'list-move',
            delta: 1
        });
        expect(resolveHotkey(key({ key: 'ArrowUp' }), ctx(false))).toEqual({
            type: 'list-move',
            delta: -1
        });
    });
});
