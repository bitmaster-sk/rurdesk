import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { afterEach, describe, expect, it } from 'vitest';
import { UiModule } from '../../ui.module';

const images = ['one', 'two'].map(
    name =>
        `data:image/svg+xml,${encodeURIComponent(
            `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><title>${name}</title></svg>`
        )}`
);

@Component({
    standalone: false,
    template: `
        <ui-lightbox
            [(visible)]="open"
            header="Pictures"
            [src]="src()"
            alt="picture"
            [hasMany]="true"
            (previous)="steps.push(-1)"
            (next)="steps.push(1)"
        >
            <span data-testid="missing">gone</span>
        </ui-lightbox>
    `
})
class HostComponent {
    public open = true;
    public readonly src = signal<string | null>(images[0]);
    public steps: number[] = [];
}

async function render(
    configure: (host: HostComponent) => void = (): void => {}
): Promise<ComponentFixture<HostComponent>> {
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [UiModule, TranslateModule.forRoot()],
        providers: [provideNoopAnimations()]
    }).compileComponents();
    const fixture = TestBed.createComponent(HostComponent);
    configure(fixture.componentInstance);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
}

function query<T extends Element>(testid: string): T | null {
    return document.querySelector<T>(`[data-testid="${testid}"]`);
}

function zoomLevel(): string | undefined {
    return query('ui-lightbox-zoom-level')?.textContent?.trim();
}

function transform(): string | undefined {
    return query<HTMLElement>('ui-lightbox-image')?.style.transform;
}

function isZoomButtonDisabled(name: 'in' | 'out' | 'reset'): boolean | undefined {
    return query<HTMLButtonElement>(`ui-lightbox-zoom-${name}`)?.querySelector('button')?.disabled;
}

async function press(fixture: ComponentFixture<HostComponent>, key: string): Promise<void> {
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
    fixture.detectChanges();
    await fixture.whenStable();
}

describe('UiLightboxComponent', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('shows the image and asks the parent for the neighbours', async () => {
        const fixture = await render();
        expect(query<HTMLImageElement>('ui-lightbox-image')?.alt).toBe('picture');

        query<HTMLElement>('ui-lightbox-next')?.click();
        await press(fixture, 'ArrowLeft');

        expect(fixture.componentInstance.steps).toEqual([1, -1]);
    });

    it('shows the projected content when there is no image', async () => {
        await render(host => host.src.set(null));

        expect(query('ui-lightbox-image')).toBeNull();
        expect(query('missing')?.textContent).toBe('gone');
        expect(query('ui-lightbox-zoom-level')).toBeNull();
    });

    it('zooms in and back to fit with the buttons and keys', async () => {
        const fixture = await render();
        expect(zoomLevel()).toBe('100 %');

        query<HTMLElement>('ui-lightbox-zoom-in')?.querySelector('button')?.click();
        fixture.detectChanges();
        expect(zoomLevel()).toBe('150 %');
        expect(transform()).toContain('scale(1.5)');

        await press(fixture, '+');
        expect(zoomLevel()).toBe('225 %');

        await press(fixture, '0');
        expect(zoomLevel()).toBe('100 %');
        expect(transform()).toContain('scale(1)');
    });

    it('zooms with the mouse wheel', async () => {
        const fixture = await render();

        query('ui-lightbox-viewport')?.dispatchEvent(
            new WheelEvent('wheel', { deltaY: -200, bubbles: true, cancelable: true })
        );
        fixture.detectChanges();

        expect(Number(zoomLevel()?.replace(' %', ''))).toBeGreaterThan(100);
    });

    it('fits a new image again after zooming', async () => {
        const fixture = await render();
        await press(fixture, '+');
        expect(zoomLevel()).toBe('150 %');

        fixture.componentInstance.src.set(images[1]);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        expect(zoomLevel()).toBe('100 %');
    });

    it('disables the zoom buttons that can do nothing', async () => {
        const fixture = await render();

        expect(isZoomButtonDisabled('out')).toBe(true);
        expect(isZoomButtonDisabled('reset')).toBe(true);
        expect(isZoomButtonDisabled('in')).toBe(false);

        for (let i = 0; i < 6; i++) {
            await press(fixture, '+');
        }

        expect(zoomLevel()).toBe('800 %');
        expect(isZoomButtonDisabled('in')).toBe(true);
        expect(isZoomButtonDisabled('out')).toBe(false);
        expect(isZoomButtonDisabled('reset')).toBe(false);
    });

    it('ignores keys while closed', async () => {
        const fixture = await render(host => (host.open = false));

        await press(fixture, 'ArrowRight');

        expect(fixture.componentInstance.steps).toEqual([]);
    });
});
