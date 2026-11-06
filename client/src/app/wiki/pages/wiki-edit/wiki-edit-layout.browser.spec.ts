import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { EditorView } from '@codemirror/view';
import { undo } from '@codemirror/commands';
import { EMPTY, Observable, Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { IssueLinkApi } from 'src/app/link/api/issue-link.api.service';
import { IssueLinkPickerComponent } from 'src/app/link/components/issue-link-picker/issue-link-picker.component';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentRenderDirective } from 'src/app/shared/attachment/directives/attachment-render.directive';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TablerIconStub, AttachmentLightboxStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiEditorComponent } from '../../components/wiki-editor/wiki-editor.component';
import { WikiTablePickerComponent } from '../../components/wiki-table-picker/wiki-table-picker.component';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiMermaidDirective } from '../../directives/wiki-mermaid.directive';
import { WikiPlantUmlDirective } from '../../directives/wiki-plantuml.directive';
import { WikiPageView } from '../../model/wiki-page.model';
import { WikiTree } from '../../model/wiki-tree.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiEditPage } from './wiki-edit.page';

@Component({ selector: 'app-wiki-tree-show', template: '', standalone: true })
class WikiTreeShowStub {}

const projectSpace = { idSpace: 2, canEdit: true, canManage: true };
const sharedSpace = { idSpace: 1, canEdit: true, canManage: false };

function pageView(): WikiPageView {
    return {
        page: {
            idPage: 40,
            idSpace: 2,
            idParent: null,
            slug: 'setup',
            title: 'Setup',
            summary: '',
            body: 'x'.repeat(20),
            agentAccess: WikiAgentAccess.OnDemand,
            rank: 'a',
            versionNo: 1,
            deletedAt: null,
            createAt: '2026-10-08T10:00:00Z',
            updateAt: '2026-10-08T10:00:00Z',
            createBy: 1,
            updateBy: 1
        },
        spaceKind: WikiSpaceKind.Project,
        canEdit: true,
        canManage: true,
        backlinks: { items: [], total: 0 },
        issues: { items: [], total: 0 },
        links: [],
        draft: null
    };
}

function tree(alwaysTokens: number, tokenLimit: number): WikiTree {
    return {
        spaces: [
            { ...projectSpace, kind: WikiSpaceKind.Project, idHomePage: null },
            { ...sharedSpace, kind: WikiSpaceKind.Instance, idHomePage: null }
        ],
        nodes: [],
        alwaysTokens,
        tokenLimit,
        trashCount: 0,
        proposalCount: 0
    };
}

interface RenderOptions {
    readonly width: number;
    readonly height?: number;
    readonly isCreate?: boolean;
    readonly tree?: WikiTree | null;
}

interface Rendered {
    readonly fixture: ComponentFixture<WikiEditPage>;
    readonly insert$: ReturnType<typeof vi.fn>;
    readonly host: HTMLElement;
}

async function render(options: RenderOptions): Promise<Rendered> {
    const insert$ = vi.fn((): Observable<unknown> => new Subject());
    const isCreate = options.isCreate ?? true;
    await TestBed.configureTestingModule({
        declarations: [
            WikiEditPage,
            WikiEditorComponent,
            WikiTablePickerComponent,
            IssueLinkPickerComponent,
            WikiMermaidDirective,
            WikiPlantUmlDirective,
            AttachmentRenderDirective
        ],
        imports: [
            MarkdownModule.forRoot(),
            TranslateModule.forRoot(),
            FormsModule,
            UiModule,
            TablerIconStub,
            AttachmentLightboxStub,
            WikiTreeShowStub
        ],
        providers: [
            {
                provide: ActivatedRoute,
                useValue: {
                    snapshot: {
                        data: { isCreate },
                        paramMap: convertToParamMap({
                            idProject: '7',
                            space: 'project',
                            slug: 'setup'
                        }),
                        queryParamMap: convertToParamMap({ title: 'Setup' })
                    }
                }
            },
            {
                provide: WikiApi,
                useValue: {
                    insert$,
                    loadOne$: () => of(pageView()),
                    saveDraft$: vi.fn(() => EMPTY),
                    deleteDraft$: vi.fn(() => EMPTY),
                    heartbeat$: vi.fn(() => of([])),
                    leave$: vi.fn(() => EMPTY)
                }
            },
            {
                provide: WikiTreeStore,
                useValue: {
                    tree: () => options.tree ?? null,
                    sharedSpace: () => sharedSpace,
                    projectSpace: () => projectSpace,
                    reload: () => undefined
                }
            },
            { provide: AuthStore, useValue: { user: () => ({ idUser: 1 }) } },
            { provide: NoticeService, useValue: { wikiPage$: EMPTY } },
            { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
            {
                provide: IssueLinkApi,
                useValue: { load$: (): Observable<unknown> => of([]) }
            },
            {
                provide: AttachmentApi,
                useValue: {
                    download$: (): Observable<Blob> => of(new Blob(['x'])),
                    upload$: (): Observable<never> => new Subject()
                }
            }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiEditPage);
    const host = fixture.nativeElement as HTMLElement;
    host.style.display = 'block';
    host.style.width = `${options.width}px`;
    host.style.height = `${options.height ?? 720}px`;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, insert$, host };
}

function byTestId(el: HTMLElement, testId: string): HTMLElement {
    const found = el.querySelector<HTMLElement>(`[data-testid="${testId}"]`);
    if (!found) {
        throw new Error(`missing [data-testid="${testId}"]`);
    }
    return found;
}

function toggleHost(el: HTMLElement): HTMLElement {
    return byTestId(el, 'wiki-edit-settings-toggle');
}

function toggleButton(el: HTMLElement): HTMLButtonElement {
    return toggleHost(el).querySelector('button') as HTMLButtonElement;
}

function saveButton(el: HTMLElement): HTMLButtonElement {
    return byTestId(el, 'wiki-save').querySelector('button') as HTMLButtonElement;
}

function splitPane(el: HTMLElement): HTMLElement {
    return byTestId(el, 'wiki-edit-split');
}

function settingsPane(el: HTMLElement): HTMLElement {
    return byTestId(el, 'wiki-edit-settings');
}

function rectOf(element: HTMLElement): DOMRect {
    return element.getBoundingClientRect();
}

function displayOf(element: HTMLElement): string {
    return getComputedStyle(element).display;
}

describe('WikiEditPage settings layout', () => {
    it('keeps the panel beside the split on desktop and behind the toggle on narrow screens', async () => {
        const { host } = await render({ width: 1280 });

        // Desktop: the toggle is hidden and the panel sits beside the split.
        expect(displayOf(toggleHost(host))).toBe('none');
        expect(displayOf(settingsPane(host))).toBe('block');
        const splitRect = rectOf(splitPane(host));
        const settingsRect = rectOf(settingsPane(host));
        expect(settingsRect.left).toBeGreaterThanOrEqual(splitRect.right - 1);
        expect(settingsRect.width).toBeLessThan(300);

        for (const width of [320, 375, 430]) {
            host.style.width = `${width}px`;

            // Closed: the panel takes no width and the split fills the editor.
            // (The toggle is a flex item of the bar, so its computed display is
            // blockified — assert visibility, not the exact keyword.)
            expect(displayOf(toggleHost(host))).not.toBe('none');
            expect(displayOf(settingsPane(host))).toBe('none');
            expect(rectOf(splitPane(host)).width).toBeCloseTo(width, 0);

            // Title, Save and Cancel stay reachable at phone widths.
            for (const testId of ['wiki-edit-title', 'wiki-save']) {
                const rect = rectOf(byTestId(host, testId));
                expect(rect.width).toBeGreaterThan(0);
                expect(rect.right).toBeLessThanOrEqual(rectOf(host).right + 1);
            }
        }
    });

    it('opens the settings in place of the editor and returns without losing anything', async () => {
        const { fixture, host } = await render({ width: 375, height: 400 });

        const view = EditorView.findFromDOM(host.querySelector<HTMLElement>('.cm-editor')!)!;
        view.dispatch({ changes: { from: 0, insert: 'Typed before switching' } });
        fixture.detectChanges();

        expect(toggleButton(host).getAttribute('aria-pressed')).toBe('false');

        toggleButton(host).click();
        fixture.detectChanges();

        // Open: the panel replaces the split full-width, fits and scrolls; the
        // editor is only hidden, never destroyed.
        expect(toggleButton(host).getAttribute('aria-pressed')).toBe('true');
        expect(displayOf(splitPane(host))).toBe('none');
        expect(displayOf(settingsPane(host))).toBe('block');
        const settingsRect = rectOf(settingsPane(host));
        expect(settingsRect.width).toBeCloseTo(375, 0);
        expect(settingsRect.right).toBeLessThanOrEqual(rectOf(host).right + 1);
        expect(getComputedStyle(settingsPane(host)).overflowY).toBe('auto');
        expect(settingsPane(host).scrollHeight).toBeGreaterThan(settingsPane(host).clientHeight);

        // The same EditorView instance survives the whole switch.
        expect(EditorView.findFromDOM(host.querySelector<HTMLElement>('.cm-editor')!)).toBe(view);

        toggleButton(host).click();
        fixture.detectChanges();

        expect(toggleButton(host).getAttribute('aria-pressed')).toBe('false');
        expect(displayOf(splitPane(host))).toBe('grid');
        expect(displayOf(settingsPane(host))).toBe('none');
        expect(host.querySelector<HTMLElement>('.cm-content')!.textContent).toContain(
            'Typed before switching'
        );

        // Undo history survived too: undo removes the inserted text.
        expect(undo(view)).toBe(true);
        expect(host.querySelector<HTMLElement>('.cm-content')!.textContent).not.toContain(
            'Typed before switching'
        );
    });

    it('keeps the over-budget reason reachable in the settings while Save is disabled', async () => {
        const { fixture, host } = await render({
            width: 375,
            isCreate: false,
            tree: tree(1000, 100)
        });

        expect(saveButton(host).disabled).toBe(false);
        expect(toggleHost(host).classList.contains('wiki-edit__settings-toggle--over')).toBe(false);

        // Switching to Always reads goes over the 100-token budget.
        toggleButton(host).click();
        fixture.detectChanges();
        byTestId(host, 'wiki-access-always').click();
        fixture.detectChanges();

        expect(saveButton(host).disabled).toBe(true);
        // The toggle stays enabled and points at the reason: the red ring
        // (`.wiki-edit__settings-toggle--over` + box-shadow in the styles).
        expect(toggleButton(host).disabled).toBe(false);
        expect(toggleHost(host).classList.contains('wiki-edit__settings-toggle--over')).toBe(true);
        expect(settingsPane(host).querySelector('.ui-field-hint--error')).not.toBeNull();

        // Back to the editor: the reason stays reachable behind the toggle.
        toggleButton(host).click();
        fixture.detectChanges();
        expect(displayOf(splitPane(host))).toBe('grid');
        expect(saveButton(host).disabled).toBe(true);
        expect(toggleButton(host).disabled).toBe(false);
        expect(toggleHost(host).classList.contains('wiki-edit__settings-toggle--over')).toBe(true);
    });

    it('preserves the settings values through the switch and saves them with the same payload', async () => {
        const { fixture, insert$, host } = await render({ width: 375 });

        toggleButton(host).click();
        fixture.detectChanges();

        const summary = settingsPane(host).querySelector<HTMLTextAreaElement>('#wiki-summary')!;
        summary.value = 'Short summary for the agent';
        summary.dispatchEvent(new Event('input'));
        const note = settingsPane(host).querySelector<HTMLTextAreaElement>('#wiki-note')!;
        note.value = 'Tweak after review';
        note.dispatchEvent(new Event('input'));
        byTestId(host, 'wiki-access-always').click();
        fixture.detectChanges();

        // Round trip: close, reopen — the typed values are still there.
        toggleButton(host).click();
        fixture.detectChanges();
        toggleButton(host).click();
        fixture.detectChanges();
        expect(settingsPane(host).querySelector<HTMLTextAreaElement>('#wiki-summary')!.value).toBe(
            'Short summary for the agent'
        );
        expect(settingsPane(host).querySelector<HTMLTextAreaElement>('#wiki-note')!.value).toBe(
            'Tweak after review'
        );

        // Save from the editor view (settings closed) sends the same payload
        // as before the panel could move around.
        toggleButton(host).click();
        fixture.detectChanges();
        saveButton(host).click();
        fixture.detectChanges();

        expect(insert$).toHaveBeenCalledTimes(1);
        expect(insert$).toHaveBeenCalledWith(7, {
            space: WikiSpaceKind.Project,
            idParent: null,
            title: 'Setup',
            summary: 'Short summary for the agent',
            body: '',
            agentAccess: WikiAgentAccess.Always,
            note: 'Tweak after review'
        });
    });
});
