import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { Observable, of, throwError } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { MessageBodyComponent } from 'src/app/shared/mention/message-body/message-body.component';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentStore } from 'src/app/shared/attachment/store/attachment.store';
import { AttachmentLightboxComponent } from 'src/app/shared/attachment/components/attachment-lightbox/attachment-lightbox.component';
import { AttachmentRenderDirective } from 'src/app/shared/attachment/directives/attachment-render.directive';

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

const idImage = '11111111-1111-1111-1111-111111111111';
const idFile = '22222222-2222-2222-2222-222222222222';
const idPdf = '33333333-3333-3333-3333-333333333333';
const idGone = '44444444-4444-4444-4444-444444444444';
const idImage2 = '55555555-5555-5555-5555-555555555555';

const blobs = new Map<string, Blob>([
    [idImage, new Blob(['png'], { type: 'image/png' })],
    [idImage2, new Blob(['png2'], { type: 'image/png' })],
    [idFile, new Blob(['x'.repeat(2048)], { type: 'application/zip' })],
    [idPdf, new Blob(['pdf'], { type: 'application/pdf' })]
]);

let lastFixture: ComponentFixture<MessageBodyComponent>;

async function render(body: string): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        imports: [
            MarkdownModule.forRoot({
                markedOptions: MARKDOWN_MARKED_OPTIONS,
                markedExtensions: [MARKDOWN_MENTION_EXTENSION]
            }),
            UiModule,
            TranslateModule.forRoot(),
            TablerIconStub,
            DiffViewerStub
        ],
        declarations: [
            MessageBodyComponent,
            MockupCardComponent,
            AttachmentRenderDirective,
            AttachmentLightboxComponent
        ],
        providers: [
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
    const fixture: ComponentFixture<MessageBodyComponent> =
        TestBed.createComponent(MessageBodyComponent);
    fixture.componentRef.setInput('body', body);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    lastFixture = fixture;
    return fixture.nativeElement as HTMLElement;
}

describe('MessageBodyComponent attachments', () => {
    it('shows an image attachment as a fixed-size thumbnail', async () => {
        const el = await render(`look ![shot.png](attachment:${idImage})`);
        const thumb = el.querySelector<HTMLElement>('[data-testid="attachment-thumb"]');
        expect(thumb).not.toBeNull();
        expect(thumb?.querySelector('img')?.getAttribute('alt')).toBe('shot.png');
        expect(thumb?.querySelector('img')?.src.startsWith('blob:')).toBe(true);
        const box = thumb?.getBoundingClientRect();
        expect([box?.width, box?.height]).toEqual([160, 120]);
    });

    it('frees the downloaded file once the message is gone', async () => {
        await render(`look ![shot.png](attachment:${idImage})`);
        const store = TestBed.inject(AttachmentStore);
        expect(store.state(idImage)?.status).toBe('ready');

        lastFixture.destroy();

        expect(store.state(idImage)).toBeNull();
    });

    it('shows a file attachment as a card with name and size', async () => {
        const el = await render(`[data.zip](attachment:${idFile})`);
        const card = el.querySelector<HTMLAnchorElement>('[data-testid="attachment-card"]');
        expect(card?.querySelector('[data-testid="attachment-card-name"]')?.textContent).toBe(
            'data.zip'
        );
        expect(card?.querySelector('[data-testid="attachment-card-size"]')?.textContent).toBe(
            '2.0 KB'
        );
        expect(card?.download).toBe('data.zip');
    });

    it('opens a pdf attachment in a new tab', async () => {
        const el = await render(`[spec.pdf](attachment:${idPdf})`);
        const card = el.querySelector<HTMLAnchorElement>('[data-testid="attachment-card"]');
        expect(card?.target).toBe('_blank');
        expect(card?.hasAttribute('download')).toBe(false);
    });

    it('shows a placeholder for a missing attachment', async () => {
        const el = await render(`![gone.png](attachment:${idGone})`);
        expect(el.querySelector('[data-testid="attachment-thumb"]')).toBeNull();
        expect(el.querySelector('[data-testid="attachment-missing"]')?.textContent).toBe(
            'ATTACHMENT.MISSING'
        );
    });

    it('leaves a hand-written marker link alone instead of fetching its path', async () => {
        const el = await render('[x](#attachment-../../admin/user/1 "attachment-file")');
        expect(el.querySelector('[data-testid="attachment-missing"]')).toBeNull();
        expect(el.querySelector('[data-testid="attachment-loading"]')).toBeNull();
        expect(el.querySelector('a')?.textContent).toBe('x');
    });
});

describe('MessageBodyComponent lightbox', () => {
    const twoImages = `![one.png](attachment:${idImage})\n\nsome text\n\n![two.png](attachment:${idImage2})`;

    function lightboxAlt(): string | null | undefined {
        return document.querySelector('[data-testid="ui-lightbox-image"]')?.getAttribute('alt');
    }

    async function settle(): Promise<void> {
        await new Promise(resolve => setTimeout(resolve, 50));
    }

    function press(key: string): void {
        document.body.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
    }

    it('opens the clicked image enlarged', async () => {
        const el = await render(twoImages);
        const thumbs = el.querySelectorAll<HTMLElement>('[data-testid="attachment-thumb"]');
        thumbs[1].click();
        await settle();
        expect(lightboxAlt()).toBe('two.png');
    });

    it('moves between the images of the message with the arrows and keys', async () => {
        const el = await render(twoImages);
        el.querySelector<HTMLElement>('[data-testid="attachment-thumb"]')?.click();
        await settle();
        expect(lightboxAlt()).toBe('one.png');

        document.querySelector<HTMLElement>('[data-testid="ui-lightbox-next"]')?.click();
        await settle();
        expect(lightboxAlt()).toBe('two.png');

        press('ArrowLeft');
        await settle();
        expect(lightboxAlt()).toBe('one.png');
    });

    it('renders the lightbox only while an image is open', async () => {
        const el = await render(twoImages);
        expect(el.querySelector('app-attachment-lightbox')).toBeNull();

        el.querySelector<HTMLElement>('[data-testid="attachment-thumb"]')?.click();
        await settle();
        expect(el.querySelector('app-attachment-lightbox')).not.toBeNull();
    });

    it('closes on Escape', async () => {
        const el = await render(twoImages);
        el.querySelector<HTMLElement>('[data-testid="attachment-thumb"]')?.click();
        await settle();
        expect(lightboxAlt()).toBe('one.png');

        press('Escape');
        await settle();
        expect(document.querySelector('[data-testid="ui-lightbox"]')).toBeNull();
        expect(el.querySelector('app-attachment-lightbox')).toBeNull();
    });
});
