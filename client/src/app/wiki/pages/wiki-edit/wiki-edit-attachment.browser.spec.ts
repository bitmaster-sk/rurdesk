import { Component, input, output, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { EMPTY, Observable, of, throwError } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentLightboxComponent } from 'src/app/shared/attachment/components/attachment-lightbox/attachment-lightbox.component';
import { AttachmentRenderDirective } from 'src/app/shared/attachment/directives/attachment-render.directive';
import { AttachmentScope } from 'src/app/shared/attachment/entity/attachment-scope.entity';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiMermaidDirective } from '../../directives/wiki-mermaid.directive';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiEditPage } from './wiki-edit.page';

@Component({ selector: 'app-wiki-tree-show', template: '', standalone: true })
class WikiTreeShowStub {}

@Component({ selector: 'app-wiki-editor', template: '', standalone: true })
class WikiEditorStub {
    public readonly initialValue = input('');
    public readonly placeholderText = input('');
    public readonly pages = input<unknown[]>([]);
    public readonly issues = input<unknown[]>([]);
    public readonly attachmentScope = input<AttachmentScope | null>(null);
    public readonly valueChange = output<string>();
    public readonly issuesRequested = output<void>();

    public readonly isUploading = signal(false);
}

const idImage = '11111111-1111-1111-1111-111111111111';
const idFile = '33333333-3333-3333-3333-333333333333';

const blobs = new Map<string, Blob>([
    [idImage, new Blob(['png'], { type: 'image/png' })],
    [idFile, new Blob(['x'.repeat(2048)], { type: 'application/zip' })]
]);

const projectSpace = { idSpace: 2, canEdit: true, canManage: true };
const sharedSpace = { idSpace: 1, canEdit: true, canManage: false };

async function render(): Promise<ComponentFixture<WikiEditPage>> {
    await TestBed.configureTestingModule({
        declarations: [
            WikiEditPage,
            WikiMermaidDirective,
            AttachmentRenderDirective,
            AttachmentLightboxComponent
        ],
        imports: [
            MarkdownModule.forRoot(),
            TranslateModule.forRoot(),
            FormsModule,
            UiModule,
            TablerIconStub,
            WikiTreeShowStub,
            WikiEditorStub
        ],
        providers: [
            {
                provide: ActivatedRoute,
                useValue: {
                    snapshot: {
                        data: { isCreate: true },
                        paramMap: convertToParamMap({ idProject: '7' }),
                        queryParamMap: convertToParamMap({ title: 'Setup' })
                    }
                }
            },
            { provide: WikiApi, useValue: {} },
            { provide: IssueApi, useValue: {} },
            {
                provide: WikiTreeStore,
                useValue: {
                    tree: () => null,
                    sharedSpace: () => sharedSpace,
                    projectSpace: () => projectSpace,
                    reload: () => undefined
                }
            },
            { provide: AuthStore, useValue: { user: () => ({ idUser: 1 }) } },
            { provide: NoticeService, useValue: { wikiPage$: EMPTY } },
            { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
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
    const fixture = TestBed.createComponent(WikiEditPage);
    fixture.detectChanges();
    await fixture.whenStable();
    return fixture;
}

describe('WikiEditPage attachments', () => {
    it('uploads into the space of the new page', async () => {
        const fixture = await render();
        const editor = fixture.debugElement.query(By.directive(WikiEditorStub))
            .componentInstance as WikiEditorStub;

        expect(editor.attachmentScope()).toEqual({ idSpace: 2 });
    });

    it('shows an image thumbnail and a file card in the preview', async () => {
        const fixture = await render();
        const editor = fixture.debugElement.query(By.directive(WikiEditorStub))
            .componentInstance as WikiEditorStub;

        editor.valueChange.emit(
            `![diagram.png](attachment:${idImage})\n\n[data.zip](attachment:${idFile})`
        );
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const preview = (fixture.nativeElement as HTMLElement).querySelector('.wiki-article')!;
        expect(
            preview.querySelector('[data-testid="attachment-thumb"] img')?.getAttribute('alt')
        ).toBe('diagram.png');
        expect(preview.querySelector('[data-testid="attachment-card-name"]')?.textContent).toBe(
            'data.zip'
        );
    });

    it('does not save while a file is uploading', async () => {
        const fixture = await render();
        const save = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="wiki-save"] button'
        ) as HTMLButtonElement;
        expect(save.disabled).toBe(false);

        editorStub(fixture).isUploading.set(true);
        fixture.detectChanges();

        expect(save.disabled).toBe(true);
    });

    it('locks the space once the page links a file', async () => {
        const fixture = await render();
        const trigger = (): HTMLElement =>
            (fixture.nativeElement as HTMLElement).querySelector(
                '[data-testid="wiki-edit-space"] .ui-select-trigger'
            ) as HTMLElement;
        expect(trigger().classList).not.toContain('ui-select-trigger--disabled');

        editorStub(fixture).valueChange.emit(`![diagram.png](attachment:${idImage})`);
        fixture.detectChanges();

        expect(trigger().classList).toContain('ui-select-trigger--disabled');
    });

    it('locks the space while a file is uploading', async () => {
        const fixture = await render();

        editorStub(fixture).isUploading.set(true);
        fixture.detectChanges();

        expect(
            (fixture.nativeElement as HTMLElement).querySelector(
                '[data-testid="wiki-edit-space"] .ui-select-trigger'
            )?.classList
        ).toContain('ui-select-trigger--disabled');
    });
});

function editorStub(fixture: ComponentFixture<WikiEditPage>): WikiEditorStub {
    return fixture.debugElement.query(By.directive(WikiEditorStub))
        .componentInstance as WikiEditorStub;
}
