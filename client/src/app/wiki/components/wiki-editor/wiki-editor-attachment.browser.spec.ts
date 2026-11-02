import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { EditorView } from '@codemirror/view';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { AttachmentScope } from 'src/app/shared/attachment/entity/attachment-scope.entity';
import { UploadedAttachmentLink } from 'src/app/shared/attachment/entity/uploaded-attachment-link.entity';
import { AttachmentUploader } from 'src/app/shared/attachment/service/attachment-uploader.service';
import { IssueLinkApi } from 'src/app/link/api/issue-link.api.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { TablerIconStub, UiButtonStub, UiLoaderStub, UiTooltipStub } from 'src/testing/stubs';
import { IssueLinkPickerComponent } from 'src/app/link/components/issue-link-picker/issue-link-picker.component';
import { WikiTablePickerComponent } from '../wiki-table-picker/wiki-table-picker.component';
import { WikiEditorComponent } from './wiki-editor.component';

@Component({ selector: 'ui-popover', template: '<ng-content></ng-content>', standalone: true })
class UiPopoverStub {
    public toggle(): void {}
    public hide(): void {}
}

@Component({ selector: 'ui-menu', template: '', standalone: true })
class UiMenuStub {
    public readonly model = input<UiMenuItem[]>([]);
    public toggle(): void {}
}

const idAttachment = '0b6f1c1e-7a52-4d4b-9a51-2f7c3f0d9e11';
const uploaded: UploadedAttachmentLink = {
    name: 'shot.png',
    markdown: `![shot.png](attachment:${idAttachment})`
};

interface Rendered {
    fixture: ComponentFixture<WikiEditorComponent>;
    view: EditorView;
    values: string[];
    upload: ReturnType<typeof vi.fn>;
    responses: Subject<UploadedAttachmentLink>[];
}

async function render(text: string, scope: AttachmentScope | null): Promise<Rendered> {
    const responses: Subject<UploadedAttachmentLink>[] = [];
    const upload = vi.fn(() => {
        const response = new Subject<UploadedAttachmentLink>();
        responses.push(response);
        return response;
    });
    await TestBed.configureTestingModule({
        declarations: [WikiEditorComponent, WikiTablePickerComponent, IssueLinkPickerComponent],
        imports: [
            TranslateModule.forRoot(),
            TablerIconStub,
            UiButtonStub,
            UiTooltipStub,
            UiLoaderStub,
            UiPopoverStub,
            UiMenuStub
        ],
        providers: [
            { provide: AttachmentUploader, useValue: { upload$: upload } },
            { provide: IssueLinkApi, useValue: { load$: () => new Subject() } }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiEditorComponent);
    fixture.componentRef.setInput('initialValue', text);
    fixture.componentRef.setInput('attachmentScope', scope);
    const values: string[] = [];
    fixture.componentInstance.valueChange.subscribe(value => values.push(value));
    fixture.detectChanges();
    const element = fixture.nativeElement as HTMLElement;
    const view = EditorView.findFromDOM(element.querySelector<HTMLElement>('.cm-editor')!)!;
    view.dispatch({ selection: { anchor: text.length } });
    return { fixture, view, values, upload, responses };
}

function pngFile(name = 'shot.png'): File {
    return new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], name, { type: 'image/png' });
}

function pasteFiles(view: EditorView, ...files: File[]): void {
    pasteData(view, '', ...files);
}

function pasteData(view: EditorView, text: string, ...files: File[]): void {
    const data = new DataTransfer();
    for (const file of files) {
        data.items.add(file);
    }
    if (text) {
        data.setData('text/plain', text);
        data.setData('text/html', `<table><tr><td>${text}</td></tr></table>`);
    }
    view.contentDOM.dispatchEvent(
        new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true })
    );
}

describe('WikiEditorComponent attachments', () => {
    it('inserts a placeholder for a pasted image and swaps it for the link', async () => {
        const { fixture, view, values, upload, responses } = await render('see ', { idSpace: 4 });

        pasteFiles(view, pngFile());

        expect(upload).toHaveBeenCalledWith(expect.any(File), { idSpace: 4 });
        expect(view.state.doc.toString()).toBe('see [WIKI.ATTACHMENT.UPLOADING]');
        expect(fixture.componentInstance.isUploading()).toBe(true);

        view.dispatch({ changes: { from: 0, insert: 'Look, ' } });
        responses[0].next(uploaded);

        expect(view.state.doc.toString()).toBe(`Look, see ${uploaded.markdown}`);
        expect(values.at(-1)).toBe(`Look, see ${uploaded.markdown}`);
        expect(fixture.componentInstance.isUploading()).toBe(false);
    });

    it('keeps several uploads apart', async () => {
        const { view, responses } = await render('', { idSpace: 4 });

        pasteFiles(view, pngFile('one.png'), pngFile('two.png'));
        responses[1].next({ name: 'two.png', markdown: '![two.png](attachment:2)' });
        responses[0].next({ name: 'one.png', markdown: '![one.png](attachment:1)' });

        expect(view.state.doc.toString()).toBe('![one.png](attachment:1)![two.png](attachment:2)');
    });

    it('removes the placeholder when the upload fails', async () => {
        const { fixture, view, responses } = await render('text ', { idSpace: 4 });

        pasteFiles(view, pngFile());
        responses[0].error(new Error('too large'));

        expect(view.state.doc.toString()).toBe('text ');
        expect(fixture.componentInstance.isUploading()).toBe(false);
    });

    it('uploads files picked with the paperclip button', async () => {
        const { fixture, view, upload, responses } = await render('', { idSpace: 4 });
        const input = (fixture.nativeElement as HTMLElement).querySelector<HTMLInputElement>(
            '[data-testid="wiki-attach-input"]'
        )!;
        const data = new DataTransfer();
        data.items.add(pngFile());
        input.files = data.files;

        input.dispatchEvent(new Event('change'));
        responses[0].next(uploaded);

        expect(upload).toHaveBeenCalledTimes(1);
        expect(view.state.doc.toString()).toBe(uploaded.markdown);
    });

    it('offers no upload without a space', async () => {
        const { fixture, view, upload } = await render('', null);

        pasteFiles(view, pngFile());

        expect(upload).not.toHaveBeenCalled();
        expect(
            (fixture.nativeElement as HTMLElement).querySelector(
                '[data-testid="wiki-format-attach"]'
            )
        ).toBeNull();
    });

    it('pastes the text when the clipboard also carries a picture of it', async () => {
        const { view, upload } = await render('', { idSpace: 4 });

        pasteData(view, 'A1\tB1', pngFile());

        expect(upload).not.toHaveBeenCalled();
        expect(view.state.doc.toString()).toBe('A1\tB1');
    });
});
