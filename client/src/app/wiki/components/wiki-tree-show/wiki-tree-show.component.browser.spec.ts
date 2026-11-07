import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
import { WikiLayoutStore } from '../../store/wiki-layout.store';
import { WikiTreeShowComponent } from './wiki-tree-show.component';

async function render(): Promise<{ element: HTMLElement; detect: () => void }> {
    await TestBed.configureTestingModule({
        declarations: [WikiTreeShowComponent],
        imports: [TranslateModule.forRoot(), TablerIconStub, UiButtonStub, UiTooltipStub]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiTreeShowComponent);
    fixture.detectChanges();
    return {
        element: fixture.nativeElement as HTMLElement,
        detect: () => fixture.detectChanges()
    };
}

describe('WikiTreeShowComponent', () => {
    beforeEach(() =>
        vi.spyOn(window, 'matchMedia').mockReturnValue({
            matches: false,
            addEventListener: () => undefined
        } as unknown as MediaQueryList)
    );
    afterEach(() => {
        localStorage.removeItem('wiki.treeHidden');
        vi.restoreAllMocks();
    });

    it('shows nothing while the page tree is visible', async () => {
        localStorage.removeItem('wiki.treeHidden');
        const { element } = await render();
        expect(element.querySelector('[data-testid="wiki-tree-show"]')).toBeNull();
    });

    it('offers a button that brings a hidden page tree back', async () => {
        localStorage.setItem('wiki.treeHidden', 'true');
        const { element, detect } = await render();
        const button = element.querySelector<HTMLElement>('[data-testid="wiki-tree-show"]');
        expect(button).not.toBeNull();

        button?.click();
        detect();

        expect(TestBed.inject(WikiLayoutStore).isTreeHidden()).toBe(false);
        expect(element.querySelector('[data-testid="wiki-tree-show"]')).toBeNull();
    });
});
