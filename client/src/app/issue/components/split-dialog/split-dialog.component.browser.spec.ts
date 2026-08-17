import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { Component, input, output } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';

import { UiDialogStub } from 'src/testing/stubs';
import { SplitDialogComponent } from './split-dialog.component';

@Component({ selector: 'app-split-input-step', template: '', standalone: true })
class SplitInputStepStub {
    public readonly isLoading = input<boolean>(false);
    public readonly issueTitle = input<string>('');
    public readonly split = output<string>();
    public readonly cancelled = output<void>();
}

@Component({ selector: 'app-split-review-step', template: '', standalone: true })
class SplitReviewStepStub {
    public readonly initialChildren = input<unknown[]>([]);
    public readonly isSaving = input<boolean>(false);
    public readonly severities = input<unknown[]>([]);
    public readonly states = input<unknown[]>([]);
    public readonly accept = output<unknown[]>();
    public readonly cancelled = output<void>();
}

@Component({ selector: 'app-split-done-step', template: '', standalone: true })
class SplitDoneStepStub {
    public readonly count = input<number>(0);
    public readonly closed = output<void>();
}
import { SplitApi } from '../../api/split.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { ProjectStore } from 'src/app/project/project.store';
import { Issue } from '../../model/issue.model';
import { ProposedIssue } from '../../model/split.model';

describe('SplitDialogComponent', () => {
    let component: SplitDialogComponent;
    let fixture: ComponentFixture<SplitDialogComponent>;
    let mockSplitApi: any;
    let mockToast: any;

    const mockIssue: Issue = {
        idIssue: 10,
        idIssuePublic: 2,
        idProject: 1,
        idState: 5,
        idSeverity: 3,
        title: 'Test Issue',
        description: 'Test description',
        tracked: 0
    };

    const apiChildren: ProposedIssue[] = [
        { title: 'Child 1', description: 'Desc 1', idSeverity: null, idState: null },
        { title: 'Child 2', description: 'Desc 2', idSeverity: null, idState: null }
    ];

    beforeEach(async () => {
        mockSplitApi = { preview$: vi.fn(), accept$: vi.fn() };
        mockToast = { showError: vi.fn() };

        await TestBed.configureTestingModule({
            declarations: [SplitDialogComponent],
            imports: [
                TranslateModule.forRoot(),
                UiDialogStub,
                SplitInputStepStub,
                SplitReviewStepStub,
                SplitDoneStepStub
            ],
            providers: [
                { provide: SplitApi, useValue: mockSplitApi },
                { provide: ToastNotificationService, useValue: mockToast },
                { provide: SeverityStore, useValue: { severitiesByProject$: () => of([]) } },
                { provide: StateStore, useValue: { statesByProject$: () => of([]) } },
                { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(SplitDialogComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('idProject', 1);
        fixture.componentRef.setInput('issue', mockIssue);
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('starts on input step', () => {
        expect(component.step()).toBe('input');
    });

    it('onSplit() sets step to loading and calls preview$', () => {
        mockSplitApi.preview$.mockReturnValue(of({ children: apiChildren }));
        component.onSplit('split hint');
        expect(mockSplitApi.preview$).toHaveBeenCalledWith(1, 2, 'split hint');
    });

    it('after preview$, step is review and children inherit parent severity/state', () => {
        mockSplitApi.preview$.mockReturnValue(of({ children: apiChildren }));
        component.onSplit('');
        expect(component.step()).toBe('review');
        expect(component.children()[0].idSeverity).toBe(3);
        expect(component.children()[0].idState).toBe(5);
    });

    it('a 429 error from preview$ shows ERROR_RATE_LIMIT toast and returns to input', () => {
        mockSplitApi.preview$.mockReturnValue(throwError(() => ({ status: 429 })));
        component.onSplit('');
        expect(mockToast.showError).toHaveBeenCalledWith('SPLIT.ERROR_RATE_LIMIT');
        expect(component.step()).toBe('input');
    });

    it('a 503 error from preview$ shows ERROR_AI toast and returns to input', () => {
        mockSplitApi.preview$.mockReturnValue(throwError(() => ({ status: 503 })));
        component.onSplit('');
        expect(mockToast.showError).toHaveBeenCalledWith('SPLIT.ERROR_AI');
        expect(component.step()).toBe('input');
    });

    it('onAccept() calls accept$ with the provided children', () => {
        const acceptedChildren: ProposedIssue[] = [
            { title: 'C1', description: '', idSeverity: null, idState: null }
        ];
        const returnedIssues: Issue[] = [
            {
                idIssue: 11,
                idIssuePublic: 3,
                idProject: 1,
                idState: null,
                idSeverity: null,
                title: 'C1',
                description: '',
                tracked: 0
            }
        ];
        mockSplitApi.accept$.mockReturnValue(of({ children: returnedIssues }));
        component.onAccept(acceptedChildren);
        expect(mockSplitApi.accept$).toHaveBeenCalledWith(1, 2, acceptedChildren);
    });

    it('after accept$, step is done and acceptedCount is set', () => {
        const returnedIssues: Issue[] = [
            {
                idIssue: 11,
                idIssuePublic: 3,
                idProject: 1,
                idState: null,
                idSeverity: null,
                title: 'C1',
                description: '',
                tracked: 0
            }
        ];
        mockSplitApi.accept$.mockReturnValue(of({ children: returnedIssues }));
        component.onAccept([{ title: 'C1', description: '', idSeverity: null, idState: null }]);
        expect(component.step()).toBe('done');
        expect(component.acceptedCount()).toBe(1);
    });

    it('an error from accept$ shows ERROR_AI toast and returns to review', () => {
        mockSplitApi.accept$.mockReturnValue(throwError(() => ({ status: 500 })));
        component.onAccept([{ title: 'C1', description: '', idSeverity: null, idState: null }]);
        expect(mockToast.showError).toHaveBeenCalledWith('SPLIT.ERROR_AI');
        expect(component.step()).toBe('review');
    });

    it('onCancel() emits cancelled', () => {
        let emitted = false;
        component.cancelled.subscribe(() => (emitted = true));
        component.onCancel();
        expect(emitted).toBe(true);
    });
});
