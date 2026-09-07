import { describe, it, expect } from 'vitest';
import { KeyboardResolver } from './keyboard-resolver';

const key = (init: Partial<KeyboardEvent> & { key: string }): KeyboardEvent =>
    ({
        metaKey: false,
        ctrlKey: false,
        isComposing: false,
        target: null,
        ...init
    }) as unknown as KeyboardEvent;

const target = (init: Record<string, unknown>): EventTarget =>
    ({ closest: () => null, ...init }) as unknown as EventTarget;

describe('KeyboardResolver.resolveHotkey', () => {
    const ctx = (paletteOpen = false, helpOpen = false) => ({ paletteOpen, helpOpen });
    it('Cmd+K opens (all) / closes when open', () => {
        expect(
            KeyboardResolver.resolveHotkey(key({ key: 'k', metaKey: true }), ctx(false))
        ).toEqual({
            type: 'open',
            mode: 'all'
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: 'k', metaKey: true }), ctx(true))).toEqual(
            {
                type: 'close'
            }
        );
    });
    it('ignores everything else while the palette is open', () => {
        expect(KeyboardResolver.resolveHotkey(key({ key: '/' }), ctx(true))).toEqual({
            type: 'none'
        });
    });
    it('ignores bare keys while the help sheet is open', () => {
        expect(KeyboardResolver.resolveHotkey(key({ key: '/' }), ctx(false, true))).toEqual({
            type: 'none'
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: '?' }), ctx(false, true))).toEqual({
            type: 'none'
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: 'j' }), ctx(false, true))).toEqual({
            type: 'none'
        });
    });
    it('never fires bare keys in an editable target or during IME', () => {
        for (const tagName of ['INPUT', 'TEXTAREA', 'SELECT']) {
            expect(
                KeyboardResolver.resolveHotkey(
                    key({ key: '/', target: target({ tagName }) }),
                    ctx()
                )
            ).toEqual({ type: 'none' });
        }
        expect(
            KeyboardResolver.resolveHotkey(
                key({ key: '/', target: target({ tagName: 'DIV', isContentEditable: true }) }),
                ctx()
            )
        ).toEqual({ type: 'none' });
        expect(KeyboardResolver.resolveHotkey(key({ key: '/', isComposing: true }), ctx())).toEqual(
            {
                type: 'none'
            }
        );
    });
    it('fires bare keys from a plain non-editable target', () => {
        expect(
            KeyboardResolver.resolveHotkey(
                key({ key: '/', target: target({ tagName: 'DIV', isContentEditable: false }) }),
                ctx()
            )
        ).toEqual({ type: 'open', mode: 'navigation' });
    });
    // Regression: arrows inside an open dropdown moved both its highlight and the
    // task row behind it.
    it('never fires bare keys from inside a CDK overlay', () => {
        const inOverlay = {
            tagName: 'BUTTON',
            closest: (sel: string) => (sel === '.cdk-overlay-container' ? {} : null)
        } as unknown as EventTarget;
        expect(
            KeyboardResolver.resolveHotkey(key({ key: 'ArrowDown', target: inOverlay }), ctx(false))
        ).toEqual({
            type: 'none'
        });
        expect(
            KeyboardResolver.resolveHotkey(key({ key: '/', target: inOverlay }), ctx(false))
        ).toEqual({
            type: 'none'
        });
    });
    it('bare / opens navigation, ? opens help, j/k & arrows move the list', () => {
        expect(KeyboardResolver.resolveHotkey(key({ key: '/' }), ctx(false))).toEqual({
            type: 'open',
            mode: 'navigation'
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: '?' }), ctx(false))).toEqual({
            type: 'help'
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: 'j' }), ctx(false))).toEqual({
            type: 'list-move',
            delta: 1
        });
        expect(KeyboardResolver.resolveHotkey(key({ key: 'ArrowUp' }), ctx(false))).toEqual({
            type: 'list-move',
            delta: -1
        });
    });
});
