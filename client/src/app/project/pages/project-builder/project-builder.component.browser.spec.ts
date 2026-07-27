import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { FormsModule } from '@angular/forms';
import { of, throwError } from 'rxjs';
import { Component, input, model, output, signal } from '@angular/core';

import { TranslateModule } from '@ngx-translate/core';
import { UiLoaderStub } from 'src/testing/stubs';
import { ProjectBuilderComponent } from './project-builder.component';

@Component({ selector: 'app-project-builder-step-input', template: '', standalone: true })
class StepInputStub {
    public readonly description = model<string>('');
    public readonly defaultIdState = model<number | null>(null);
    public readonly defaultIdSeverity = model<number | null>(null);
    public readonly states = input<unknown[]>([]);
    public readonly severities = input<unknown[]>([]);
    public readonly isGenerating = input<boolean>(false);
    public readonly isGenerateDisabled = input<boolean>(false);
    public readonly minChars = input<number>(0);
    public readonly rateLimitCountdown = input<number | null>(null);
    public readonly generate = output<void>();
}

@Component({ selector: 'app-project-builder-step-staging', template: '', standalone: true })
class StepStagingStub {
    public readonly stagedIssues = input<unknown[]>([]);
    public readonly summary = input<string>('');
    public readonly states = input<unknown[]>([]);
    public readonly severities = input<unknown[]>([]);
    public readonly isAccepting = input<boolean>(false);
    public readonly isRestoreBannerVisible = input<boolean>(false);
    public readonly restoredFrom = input<string>('');
    public readonly flatCount = input<number>(0);
    public readonly accept = output<void>();
    public readonly back = output<void>();
    public readonly regenerate = output<void>();
    public readonly issueChange = output<unknown>();
    public readonly deleteNode = output<unknown>();
    public readonly dismissBanner = output<void>();
    public readonly discardStaging = output<void>();
}

@Component({ selector: 'app-project-builder-step-success', template: '', standalone: true })
class StepSuccessStub {
    public readonly createdCount = input<number>(0);
    public readonly goToIssues = output<void>();
}
import { StagedIssuesTree } from './staged-issues-tree';
import { ProjectBuilderApi } from '../../api/project-builder.api.service';
import { ProjectStore } from '../../project.store';
import { StateStore } from '../../../state/store/state.store';
import { SeverityStore } from '../../../severity/store/severity.store';
import { ProjectBuilderIssue } from '../../model/project-builder.model';

describe('ProjectBuilderComponent', () => {
    let component: ProjectBuilderComponent;
    let fixture: ComponentFixture<ProjectBuilderComponent>;
    let mockApi: any;

    const mockProject = {
        idProject: 1,
        name: 'Test',
        idStateDefault: null,
        idSeverityDefault: null
    };

    beforeEach(async () => {
        mockApi = { generate$: vi.fn(), accept$: vi.fn() };

        await TestBed.configureTestingModule({
            declarations: [ProjectBuilderComponent],
            imports: [
                RouterTestingModule,
                HttpClientTestingModule,
                FormsModule,
                TranslateModule.forRoot(),
                UiLoaderStub,
                StepInputStub,
                StepStagingStub,
                StepSuccessStub
            ],
            providers: [
                { provide: ProjectBuilderApi, useValue: mockApi },
                {
                    provide: ProjectStore,
                    useValue: { project$: of(mockProject) }
                },
                {
                    provide: StateStore,
                    useValue: { statesByProject$: () => of([]) }
                },
                {
                    provide: SeverityStore,
                    useValue: { severitiesByProject$: () => of([]) }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(ProjectBuilderComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('should render step 1 by default when no staging data', () => {
        expect(component.step()).toBe(1);
    });

    it('should disable generate button when description is less than 10 chars', () => {
        component.description.set('short');
        expect(component.isGenerateDisabled()).toBe(true);
    });

    it('should enable generate button when description has 10+ chars', () => {
        component.description.set('Long enough description here');
        expect(component.isGenerateDisabled()).toBe(false);
    });

    it('should transition to step 2 after successful generate', () => {
        component.description.set(
            'A detailed project description that is long enough to pass validation.'
        );
        mockApi.generate$.mockReturnValue(of({ summary: 'test summary', issues: [] }));
        component.onGenerate();
        expect(component.step()).toBe(2);
        expect(component.summary()).toBe('test summary');
    });

    it('should set rate limit countdown on 429 response', () => {
        component.description.set(
            'A detailed project description that is long enough to pass validation.'
        );
        const headers = new Headers({ 'Retry-After': '15' });
        mockApi.generate$.mockReturnValue(
            throwError(() => ({
                status: 429,
                headers: { get: (k: string) => (k === 'Retry-After' ? '15' : null) }
            }))
        );
        component.onGenerate();
        expect(component.rateLimitCountdown()).toBe(15);
    });

    it('should transition to step 3 after successful accept', () => {
        component.stagedIssues.set([]);
        mockApi.accept$.mockReturnValue(
            of({ issues: [{ idProject: 1 } as any, { idProject: 1 } as any] })
        );
        component.onAccept();
        expect(component.step()).toBe(3);
        expect(component.createdCount()).toBe(2);
    });
});

describe('toTree / fromTree utilities', () => {
    const issues: ProjectBuilderIssue[] = [
        {
            ref: 'P',
            title: 'Parent',
            description: '',
            estimatedMinutes: 0,
            idState: null,
            idSeverity: null,
            hierarchyParentRef: '',
            scheduleRelations: []
        },
        {
            ref: 'C1',
            title: 'Child 1',
            description: '',
            estimatedMinutes: 0,
            idState: null,
            idSeverity: null,
            hierarchyParentRef: 'P',
            scheduleRelations: []
        },
        {
            ref: 'C2',
            title: 'Child 2',
            description: '',
            estimatedMinutes: 0,
            idState: null,
            idSeverity: null,
            hierarchyParentRef: 'P',
            scheduleRelations: []
        }
    ];

    it('toTree builds nested structure', () => {
        const tree = StagedIssuesTree.toTree(issues);
        expect(tree.length).toBe(1);
        expect(tree[0].data!.ref).toBe('P');
        expect(tree[0].children!.length).toBe(2);
    });

    it('fromTree flattens tree back to array', () => {
        const tree = StagedIssuesTree.toTree(issues);
        const flat = StagedIssuesTree.fromTree(tree);
        expect(flat.length).toBe(3);
        const refs = flat.map(i => i.ref);
        expect(refs).toContain('P');
        expect(refs).toContain('C1');
        expect(refs).toContain('C2');
    });

    it('fromTree preserves hierarchy_parent_ref', () => {
        const tree = StagedIssuesTree.toTree(issues);
        const flat = StagedIssuesTree.fromTree(tree);
        const child = flat.find(i => i.ref === 'C1')!;
        expect(child.hierarchyParentRef).toBe('P');
    });
});
