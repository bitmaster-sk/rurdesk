import { Component, inject } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { Observable, of, throwError } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { afterEach, describe, expect, it } from 'vitest';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentLightboxComponent } from 'src/app/shared/attachment/components/attachment-lightbox/attachment-lightbox.component';
import { GalleryImage } from 'src/app/shared/attachment/entity/gallery-image.entity';
import { AttachmentGallery } from 'src/app/shared/attachment/service/attachment-gallery.service';
import { AttachmentStore } from 'src/app/shared/attachment/store/attachment.store';
import { UiModule } from 'src/app/ui/ui.module';

const idOne = '11111111-1111-1111-1111-111111111111';
const idTwo = '22222222-2222-2222-2222-222222222222';
const idThree = '33333333-3333-3333-3333-333333333333';
const idGone = '44444444-4444-4444-4444-444444444444';

const blobs = new Map<string, Blob>([
    [idOne, new Blob(['one'], { type: 'image/png' })],
    [idTwo, new Blob(['two'], { type: 'image/png' })],
    [idThree, new Blob(['three'], { type: 'image/png' })]
]);

const one: GalleryImage = { idAttachment: idOne, name: 'one.png' };
const two: GalleryImage = { idAttachment: idTwo, name: 'two.png' };
const three: GalleryImage = { idAttachment: idThree, name: 'three.png' };
const gone: GalleryImage = { idAttachment: idGone, name: 'gone.png' };

// The host plays one rendered message body: it owns a real AttachmentGallery and,
// like appAttachmentRender, acquires every image it registers from the stubbed API.
@Component({
    standalone: false,
    providers: [AttachmentGallery],
    template: `
        @if (gallery.isOpen()) {
            <app-attachment-lightbox />
        }
    `
})
class HostComponent {
    protected readonly gallery = inject(AttachmentGallery);
    private readonly store = inject(AttachmentStore);

    public register(images: GalleryImage[]): void {
        this.gallery.register(this, images);
        for (const image of images) {
            this.store.acquire(image.idAttachment);
        }
    }

    public unregister(): void {
        this.gallery.unregister(this);
    }

    public open(idAttachment: string): void {
        this.gallery.open(idAttachment);
    }
}

async function render(): Promise<ComponentFixture<HostComponent>> {
    await TestBed.configureTestingModule({
        declarations: [HostComponent, AttachmentLightboxComponent],
        imports: [UiModule, TranslateModule.forRoot()],
        providers: [
            provideNoopAnimations(),
            {
                provide: AttachmentApi,
                useValue: {
                    download$: (id: string): Observable<Blob> => {
                        const blob = blobs.get(id);
                        return blob ? of(blob) : throwError(() => new Error('404'));
                    }
                }
            }
        ]
    }).compileComponents();
    const translate = TestBed.inject(TranslateService);
    translate.setTranslation('en', en);
    translate.use('en');
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
}

async function settle(): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 50));
}

function counterElement(): HTMLElement | null {
    return document.querySelector<HTMLElement>('[data-testid="ui-lightbox-counter"]');
}

function counter(): string | null {
    return counterElement()?.textContent?.trim() ?? null;
}

function lightboxAlt(): string | null | undefined {
    return document.querySelector('[data-testid="ui-lightbox-image"]')?.getAttribute('alt');
}

function missingText(): string | null {
    const element = document.querySelector('[data-testid="attachment-lightbox-missing"]');
    return element?.textContent?.trim() ?? null;
}

function click(testid: string): void {
    document.querySelector<HTMLElement>(`[data-testid="${testid}"]`)?.click();
}

describe('AttachmentLightboxComponent counter', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('shows the position of the open image and follows navigation', async () => {
        const fixture = await render();
        fixture.componentInstance.register([one, two, three]);
        fixture.componentInstance.open(idTwo);
        fixture.detectChanges();
        await settle();

        expect(counter()).toBe('2 / 3');
        expect(counterElement()?.getAttribute('aria-label')).toBe('Image 2 of 3');
        expect(lightboxAlt()).toBe('two.png');

        click('ui-lightbox-next');
        await settle();
        expect(counter()).toBe('3 / 3');

        click('ui-lightbox-next');
        await settle();
        expect(counter()).toBe('1 / 3');

        click('ui-lightbox-prev');
        await settle();
        expect(counter()).toBe('3 / 3');
    });

    it('updates the counter when the registered set changes and hides it when the group goes away', async () => {
        const fixture = await render();
        fixture.componentInstance.register([one, two, three]);
        fixture.componentInstance.open(idTwo);
        fixture.detectChanges();
        await settle();
        expect(counter()).toBe('2 / 3');

        fixture.componentInstance.register([one, two, three, gone]);
        fixture.detectChanges();
        await settle();
        expect(counter()).toBe('2 / 4');

        fixture.componentInstance.unregister();
        fixture.detectChanges();
        await settle();

        expect(counterElement()).toBeNull();
        expect(document.querySelector('[data-testid="ui-lightbox"]')).toBeNull();
        expect(document.body.textContent).not.toContain('NaN');
        expect(document.body.textContent).not.toContain('0 /');
    });

    it('shows 1 / 1 and no arrows for a single image', async () => {
        const fixture = await render();
        fixture.componentInstance.register([one]);
        fixture.componentInstance.open(idOne);
        fixture.detectChanges();
        await settle();

        expect(counter()).toBe('1 / 1');
        expect(document.querySelector('[data-testid="ui-lightbox-prev"]')).toBeNull();
        expect(document.querySelector('[data-testid="ui-lightbox-next"]')).toBeNull();
        expect(lightboxAlt()).toBe('one.png');
    });

    it('keeps the counter above the missing placeholder when the download fails', async () => {
        const fixture = await render();
        fixture.componentInstance.register([one, gone, three]);
        fixture.componentInstance.open(idGone);
        fixture.detectChanges();
        await settle();

        expect(counter()).toBe('2 / 3');
        expect(document.querySelector('[data-testid="ui-lightbox-image"]')).toBeNull();
        expect(missingText()).toBe('Attachment unavailable');
    });

    it('hides the counter when the lightbox closes', async () => {
        const fixture = await render();
        fixture.componentInstance.register([one, two, three]);
        fixture.componentInstance.open(idOne);
        fixture.detectChanges();
        await settle();
        expect(counter()).toBe('1 / 3');

        document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        await settle();

        expect(counterElement()).toBeNull();
        expect(document.querySelector('[data-testid="ui-lightbox"]')).toBeNull();
    });
});
