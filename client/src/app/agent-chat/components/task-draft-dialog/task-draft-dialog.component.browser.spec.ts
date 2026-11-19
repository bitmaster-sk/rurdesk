import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { provideRouter, Router } from '@angular/router';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { of } from 'rxjs';
import { page } from '@vitest/browser/context';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { UiModule } from '../../../ui/ui.module';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { CreateIssueReq } from 'src/app/issue/model/issue.model';
import { StateStore } from 'src/app/state/store/state.store';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub, UiButtonStub, UiDialogStub, UiSelectStub } from 'src/testing/stubs';
import { TaskDraftDialogComponent } from './task-draft-dialog.component';

const DRAFT = '# Keep the worktree\n\nKeep it until Restart.\n\n- retention removes it';
const LINK_LINE = '\n\n---\nCreated from a [conversation with the agent](/agent-chat/4)';

describe('TaskDraftDialogComponent (browser)', () => {
    let fixture: ComponentFixture<TaskDraftDialogComponent>;
    let insert: ReturnType<typeof vi.fn>;
    let navigate: ReturnType<typeof vi.spyOn>;

    beforeEach(async () => {
        insert = vi.fn((req: CreateIssueReq) =>
            of(Fixtures.issue({ idProject: req.idProject, idIssuePublic: 42 }))
        );
        await TestBed.configureTestingModule({
            declarations: [TaskDraftDialogComponent],
            imports: [
                TranslateModule.forRoot(),
                ReactiveFormsModule,
                TablerIconStub,
                UiButtonStub,
                UiDialogStub,
                UiSelectStub
            ],
            providers: [
                provideRouter([]),
                { provide: IssueApi, useValue: { insert$: insert } },
                {
                    provide: StateStore,
                    useValue: {
                        states$: of([
                            {
                                idState: 21,
                                idProject: 7,
                                name: 'Done',
                                start: false,
                                final: true,
                                protected: false,
                                orderRank: 2
                            },
                            {
                                idState: 20,
                                idProject: 7,
                                name: 'New',
                                start: true,
                                final: false,
                                protected: false,
                                orderRank: 1
                            }
                        ])
                    }
                }
            ]
        }).compileComponents();
        navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            AGENT_CHAT: {
                TASK_DRAFT: { LINK_LINE: 'Created from a [conversation with the agent]({{url}})' }
            }
        });
        translate.use('en');
    });

    function render(body: string, idProject: number | null): HTMLElement {
        fixture = TestBed.createComponent(TaskDraftDialogComponent);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('idConversation', 4);
        fixture.componentRef.setInput('idProject', idProject);
        fixture.componentRef.setInput('projects', [
            Fixtures.project({ idProject: 7, name: 'rurdesk' }),
            Fixtures.project({ idProject: 8, name: 'gateway' }),
            Fixtures.project({
                idProject: 9,
                name: 'billing',
                idStateDefault: 31,
                idSeverityDefault: 5,
                idIssueTypeDefault: 6
            })
        ]);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function title(host: HTMLElement): HTMLInputElement {
        return host.querySelector('[data-testid="task-draft-title"]')!;
    }

    function description(host: HTMLElement): HTMLTextAreaElement {
        return host.querySelector('[data-testid="task-draft-description"]')!;
    }

    function projectPicker(): UiSelectStub {
        return fixture.debugElement
            .query(debug => debug.attributes['data-testid'] === 'task-draft-project')
            .injector.get(UiSelectStub);
    }

    function saveButton(): { host: HTMLElement; stub: UiButtonStub } {
        const debug = fixture.debugElement.query(
            element => element.attributes['data-testid'] === 'task-draft-save'
        );
        return {
            host: debug.nativeElement as HTMLElement,
            stub: debug.injector.get(UiButtonStub)
        };
    }

    it('keeps Cancel and Save in the footer, outside the scrolling body', () => {
        const host = render(DRAFT, 7);

        for (const testId of ['task-draft-cancel', 'task-draft-save']) {
            const button = host.querySelector(`[data-testid="${testId}"]`)!;
            expect(button.closest('.ui-dialog__footer')).not.toBeNull();
            expect(button.closest('.ui-dialog__content')).toBeNull();
        }
        expect(title(host).closest('.ui-dialog__content')).not.toBeNull();
    });

    it('splits the draft into the title line and the description', () => {
        const host = render(DRAFT, 7);

        expect(title(host).value).toBe('Keep the worktree');
        expect(description(host).value).toBe('Keep it until Restart.\n\n- retention removes it');
    });

    it('cuts a long title to 100 characters', () => {
        const host = render(`# ${'x'.repeat(130)}\n\nbody`, 7);

        expect(title(host).value).toBe('x'.repeat(100));
    });

    it('keeps the whole draft as the description when it has no title line', () => {
        const host = render('Just some notes.', 7);

        expect(title(host).value).toBe('');
        expect(description(host).value).toBe('Just some notes.');
    });

    it('preselects the main project of the conversation', () => {
        render(DRAFT, 7);

        expect(projectPicker().value).toBe(7);
    });

    it('does not save until a project is picked when the conversation has none', () => {
        render(DRAFT, null);

        expect(saveButton().stub.disabled()).toBe(true);
        saveButton().host.click();
        fixture.detectChanges();
        expect(insert).not.toHaveBeenCalled();

        projectPicker().pick(8);
        fixture.detectChanges();

        expect(saveButton().stub.disabled()).toBe(false);
        saveButton().host.click();
        expect(insert).toHaveBeenCalledWith(expect.objectContaining({ idProject: 8 }));
    });

    it('saves the edited task with a link back to the conversation and opens it', () => {
        const host = render(DRAFT, 7);

        title(host).value = 'Keep the worktree longer';
        title(host).dispatchEvent(new Event('input'));
        fixture.detectChanges();
        saveButton().host.click();

        expect(insert).toHaveBeenCalledWith(
            expect.objectContaining({
                idProject: 7,
                idState: 20,
                idSeverity: null,
                idIssueType: null,
                title: 'Keep the worktree longer',
                description: `Keep it until Restart.\n\n- retention removes it${LINK_LINE}`
            })
        );
        expect(navigate).toHaveBeenCalledWith(['/project', 7, 'issue', 42]);
    });

    it('gives the task the defaults of the picked project', () => {
        render(DRAFT, 7);

        projectPicker().pick(9);
        fixture.detectChanges();
        saveButton().host.click();

        expect(insert).toHaveBeenCalledWith(
            expect.objectContaining({
                idProject: 9,
                idState: 31,
                idSeverity: 5,
                idIssueType: 6
            })
        );
        expect(navigate).toHaveBeenCalledWith(['/project', 9, 'issue', 42]);
    });

    it('does not save without a title', () => {
        const host = render(DRAFT, 7);

        title(host).value = '  ';
        title(host).dispatchEvent(new Event('input'));
        fixture.detectChanges();
        saveButton().host.click();

        expect(insert).not.toHaveBeenCalled();
    });
});

describe('TaskDraftDialogComponent at phone width (browser, real dialog)', () => {
    beforeEach(async () => {
        await page.viewport(390, 844);
        await TestBed.configureTestingModule({
            declarations: [TaskDraftDialogComponent],
            imports: [TranslateModule.forRoot(), ReactiveFormsModule, UiModule],
            providers: [
                provideNoopAnimations(),
                provideRouter([]),
                { provide: IssueApi, useValue: { insert$: vi.fn() } },
                { provide: StateStore, useValue: { states$: of([]) } }
            ]
        }).compileComponents();
    });

    afterEach(async () => {
        await page.viewport(1280, 800);
    });

    it('fits the width and keeps Cancel and Save visible below the form', async () => {
        const fixture = TestBed.createComponent(TaskDraftDialogComponent);
        fixture.componentRef.setInput('body', DRAFT);
        fixture.componentRef.setInput('idConversation', 4);
        fixture.componentRef.setInput('idProject', 7);
        fixture.componentRef.setInput('projects', [
            Fixtures.project({ idProject: 7, name: 'rurdesk' })
        ]);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const dialog = document.querySelector<HTMLElement>('.ui-dialog')!;
        const rect = dialog.getBoundingClientRect();
        expect(rect.left).toBeGreaterThanOrEqual(0);
        expect(rect.right).toBeLessThanOrEqual(390);
        expect(rect.bottom).toBeLessThanOrEqual(844);
        const content = dialog.querySelector<HTMLElement>('.ui-dialog__content')!;
        expect(content.scrollWidth).toBeLessThanOrEqual(content.clientWidth);
        const footer = dialog.querySelector<HTMLElement>('.ui-dialog__footer')!;
        expect(footer.getBoundingClientRect().bottom).toBeLessThanOrEqual(844);
        expect(footer.getBoundingClientRect().right).toBeLessThanOrEqual(390);
        footer
            .querySelectorAll('button')
            .forEach(button => expect(button.getBoundingClientRect().width).toBeGreaterThan(0));
    });
});
