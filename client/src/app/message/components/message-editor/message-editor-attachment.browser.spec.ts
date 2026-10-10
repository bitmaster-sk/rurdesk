import { Component, viewChild } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AttachmentUploader } from 'src/app/shared/attachment/service/attachment-uploader.service';
import { UploadedAttachmentLink } from 'src/app/shared/attachment/entity/uploaded-attachment-link.entity';
import { AttachmentScope } from 'src/app/shared/attachment/entity/attachment-scope.entity';
import { MessageRecipientType } from '../../constant/message-recipient-type.enum';
import { MessageModule } from '../../message.module';
import { EditorText } from './editor-text';
import { MessageEditorComponent } from './message-editor.component';

@Component({
    standalone: false,
    template: `
        <app-message-editor
            [message]="message"
            [allowAttachments]="allowAttachments"
            [attachmentScope]="scope"
            (messageChange)="sent.push($event)"
        ></app-message-editor>
    `
})
class HostComponent {
    public readonly editor = viewChild.required(MessageEditorComponent);
    public message = '';
    public allowAttachments = true;
    public scope: AttachmentScope | null = {
        idMessageRecipientType: MessageRecipientType.project,
        idRecipient: 42
    };
    public sent: string[] = [];
}

const idAttachment = '0b6f1c1e-7a52-4d4b-9a51-2f7c3f0d9e11';

const uploaded: UploadedAttachmentLink = {
    name: 'shot.png',
    markdown: `![shot.png](attachment:${idAttachment})`
};

async function render(configure: (host: HostComponent) => void = (): void => {}): Promise<{
    fixture: ComponentFixture<HostComponent>;
    upload: ReturnType<typeof vi.fn>;
    response: Subject<UploadedAttachmentLink>;
}> {
    const response = new Subject<UploadedAttachmentLink>();
    const upload = vi.fn().mockReturnValue(response);
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [MessageModule, NoopAnimationsModule, TranslateModule.forRoot()],
        providers: [{ provide: AttachmentUploader, useValue: { upload$: upload } }]
    }).compileComponents();
    const fixture = TestBed.createComponent(HostComponent);
    configure(fixture.componentInstance);
    fixture.detectChanges();
    await fixture.whenStable();
    return { fixture, upload, response };
}

function editorEl(fixture: ComponentFixture<HostComponent>): HTMLDivElement {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '.editor__content'
    ) as HTMLDivElement;
}

function typeText(fixture: ComponentFixture<HostComponent>, text: string): HTMLDivElement {
    const editor = editorEl(fixture);
    editor.focus();
    editor.textContent = text;
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    editor.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    return editor;
}

function pngFile(): File {
    return new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'shot.png', {
        type: 'image/png'
    });
}

function paste(editor: HTMLElement, data: DataTransfer): void {
    editor.dispatchEvent(
        new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true })
    );
}

function attachButton(fixture: ComponentFixture<HostComponent>): HTMLElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '[data-testid="message-editor-attach"]'
    );
}

describe('MessageEditorComponent attachments', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('uploads a pasted image and inserts it as a chip carrying the markdown link', async () => {
        const { fixture, upload, response } = await render();
        const editor = typeText(fixture, 'look ');
        const data = new DataTransfer();
        data.items.add(pngFile());

        paste(editor, data);
        fixture.detectChanges();

        expect(upload).toHaveBeenCalledWith(expect.any(File), {
            idMessageRecipientType: MessageRecipientType.project,
            idRecipient: 42
        });
        expect(editor.querySelector('.attachment-chip--uploading')?.textContent).toContain(
            'shot.png'
        );
        expect(EditorText.serialize(editor)).toBe('look  ');

        response.next(uploaded);
        fixture.detectChanges();

        const chip = editor.querySelector('.attachment-chip') as HTMLElement;
        expect(chip.textContent).toContain('shot.png');
        expect(chip.isContentEditable).toBe(false);
        expect(editor.querySelector('.attachment-chip--uploading')).toBeNull();
        expect(EditorText.serialize(editor)).toBe(`look ![shot.png](attachment:${idAttachment}) `);

        fixture.componentInstance.editor().onSend();
        expect(fixture.componentInstance.sent).toEqual([
            `look ![shot.png](attachment:${idAttachment}) `
        ]);
    });

    it('does not send while an upload is still running', async () => {
        const { fixture, response } = await render();
        const editor = typeText(fixture, 'see this ');
        const data = new DataTransfer();
        data.items.add(pngFile());
        paste(editor, data);
        fixture.detectChanges();

        const send = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="message-editor-send"] button'
        ) as HTMLButtonElement;
        expect(send.disabled).toBe(true);
        fixture.componentInstance.editor().onSend();
        expect(fixture.componentInstance.sent).toEqual([]);

        response.next(uploaded);
        fixture.detectChanges();

        expect(send.disabled).toBe(false);
        fixture.componentInstance.editor().onSend();
        expect(fixture.componentInstance.sent).toEqual([
            `see this ![shot.png](attachment:${idAttachment}) `
        ]);
    });

    it('drops the placeholder when the upload fails', async () => {
        const { fixture, response } = await render();
        const editor = typeText(fixture, 'hi ');
        const data = new DataTransfer();
        data.items.add(pngFile());

        paste(editor, data);
        response.error(new Error('too large'));
        fixture.detectChanges();

        expect(editor.querySelector('.attachment-chip')).toBeNull();
        expect(EditorText.serialize(editor)).toBe('hi  ');
    });

    it('uploads files picked with the paperclip button', async () => {
        const { fixture, upload, response } = await render();
        expect(attachButton(fixture)).not.toBeNull();
        const input = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="message-editor-file-input"]'
        ) as HTMLInputElement;
        const data = new DataTransfer();
        data.items.add(pngFile());
        input.files = data.files;

        input.dispatchEvent(new Event('change'));
        response.next(uploaded);
        fixture.detectChanges();

        expect(upload).toHaveBeenCalledTimes(1);
        expect(EditorText.serialize(editorEl(fixture))).toBe(
            `![shot.png](attachment:${idAttachment}) `
        );
    });

    it('renders an existing attachment link as a chip and round-trips it', async () => {
        const body = `see [report.pdf](attachment:${idAttachment}) ok`;
        const { fixture } = await render(host => (host.message = body));
        const editor = editorEl(fixture);

        expect(editor.querySelector('.attachment-chip')?.textContent).toContain('report.pdf');
        expect(EditorText.serialize(editor)).toBe(body);
    });

    it('keeps pasting plain text and hides the paperclip when attachments are off', async () => {
        const { fixture, upload } = await render(host => (host.allowAttachments = false));
        const editor = typeText(fixture, '');
        const data = new DataTransfer();
        data.items.add(pngFile());
        data.setData('text/plain', 'just text');

        paste(editor, data);
        editor.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();

        expect(upload).not.toHaveBeenCalled();
        expect(attachButton(fixture)).toBeNull();
        expect(EditorText.serialize(editor)).toBe('just text');
    });
});
