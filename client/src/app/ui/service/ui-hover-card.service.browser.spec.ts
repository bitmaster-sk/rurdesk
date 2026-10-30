import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { UiHoverCard } from './ui-hover-card.service';

@Component({
    selector: 'ui-test-card',
    template: '<div data-testid="hover-card">{{ label() }}</div>',
    standalone: true
})
class TestCardComponent {
    public readonly label = input('');
}

async function wait(ms: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, ms));
}

function card(): HTMLElement | null {
    return document.querySelector('[data-testid="hover-card"]');
}

describe('UiHoverCard', () => {
    let hoverCard: UiHoverCard;
    let anchor: HTMLElement;
    let other: HTMLElement;

    beforeEach(() => {
        TestBed.configureTestingModule({});
        hoverCard = TestBed.inject(UiHoverCard);
        anchor = document.createElement('a');
        anchor.textContent = '#12';
        other = document.createElement('a');
        other.textContent = '#13';
        document.body.append(anchor, other);
    });

    afterEach(() => {
        hoverCard.close();
        anchor.remove();
        other.remove();
    });

    function show(target: HTMLElement, label: string): void {
        hoverCard.show(target, TestCardComponent, ref => ref.setInput('label', label));
    }

    it('opens the card only after the pointer rests on the anchor', async () => {
        show(anchor, 'Fix login');
        expect(card()).toBeNull();

        await wait(UiHoverCard.openDelay + 50);

        expect(card()?.textContent).toBe('Fix login');
    });

    it('does not open when the pointer leaves before the delay', async () => {
        show(anchor, 'Fix login');
        hoverCard.hide(anchor);

        await wait(UiHoverCard.openDelay + 50);

        expect(card()).toBeNull();
    });

    it('closes shortly after the pointer leaves the anchor', async () => {
        show(anchor, 'Fix login');
        await wait(UiHoverCard.openDelay + 50);

        hoverCard.hide(anchor);
        expect(card()).not.toBeNull();
        await wait(UiHoverCard.closeDelay + 50);

        expect(card()).toBeNull();
    });

    it('stays open while the pointer moves from the anchor onto the card', async () => {
        show(anchor, 'Fix login');
        await wait(UiHoverCard.openDelay + 50);

        hoverCard.hide(anchor);
        card()!.closest('.cdk-overlay-pane')!.dispatchEvent(new PointerEvent('pointerenter'));
        await wait(UiHoverCard.closeDelay + 50);

        expect(card()).not.toBeNull();
    });

    it('replaces the card when the pointer rests on another anchor', async () => {
        show(anchor, 'Fix login');
        await wait(UiHoverCard.openDelay + 50);

        show(other, 'Dark mode');
        await wait(UiHoverCard.openDelay + 50);

        expect(document.querySelectorAll('[data-testid="hover-card"]')).toHaveLength(1);
        expect(card()?.textContent).toBe('Dark mode');
    });

    it('closes on Escape', async () => {
        show(anchor, 'Fix login');
        await wait(UiHoverCard.openDelay + 50);

        document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));

        expect(card()).toBeNull();
    });

    it('closes when the content holding its anchor goes away', async () => {
        const container = document.createElement('div');
        container.append(anchor);
        document.body.append(container);
        show(anchor, 'Fix login');
        await wait(UiHoverCard.openDelay + 50);

        hoverCard.closeWithin(other);
        expect(card()).not.toBeNull();
        hoverCard.closeWithin(container);

        expect(card()).toBeNull();
        container.remove();
    });
});
