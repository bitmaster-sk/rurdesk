import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, input, output } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { SplitReviewStepComponent } from './split-review-step.component';
import { ProposedIssue } from '../../../model/split.model';
import { StagedIssue } from 'src/app/shared/staged-issue/staged-issue.model';

@Component({ selector: 'app-staged-issue', template: '', standalone: true })
class StagedIssueStub {
    public readonly issue = input<unknown>(undefined);
    public readonly severities = input<unknown[]>([]);
    public readonly states = input<unknown[]>([]);
    public readonly issueChange = output<unknown>();
    public readonly delete = output<void>();
}

describe('SplitReviewStepComponent', () => {
    let component: SplitReviewStepComponent;
    let fixture: ComponentFixture<SplitReviewStepComponent>;

    const mockChildren: ProposedIssue[] = [
        { title: 'Child 1', description: 'Desc 1', idSeverity: null, idState: null },
        { title: 'Child 2', description: 'Desc 2', idSeverity: 1, idState: 2, estimatedMinutes: 90 }
    ];

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SplitReviewStepComponent],
            imports: [TranslateModule.forRoot(), TablerIconStub, UiButtonStub, StagedIssueStub]
        }).compileComponents();

        fixture = TestBed.createComponent(SplitReviewStepComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('initialChildren', mockChildren);
        fixture.componentRef.setInput('isSaving', false);
        fixture.componentRef.setInput('severities', []);
        fixture.componentRef.setInput('states', []);
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('seeds staged issues from initialChildren on first render', () => {
        expect(component.issues().length).toBe(2);
        expect(component.issues()[0].title).toBe('Child 1');
        expect(component.issues()[1].idSeverity).toBe(1);
    });

    it('defaults estimatedMinutes to 0 when the proposed issue has none', () => {
        expect(component.issues()[0].estimatedMinutes).toBe(0);
        expect(component.issues()[1].estimatedMinutes).toBe(90);
    });

    it('gives each staged issue a unique ref for tracking', () => {
        const [a, b] = component.issues();
        expect(a.ref).toBeTruthy();
        expect(a.ref).not.toBe(b.ref);
    });

    it('onAddChild appends a blank staged issue', () => {
        component.onAddChild();
        expect(component.issues().length).toBe(3);
        expect(component.issues()[2].title).toBe('');
    });

    it('onRemoveChild removes the issue with the given ref', () => {
        const removedRef = component.issues()[0].ref;
        component.onRemoveChild(removedRef);
        expect(component.issues().length).toBe(1);
        expect(component.issues()[0].title).toBe('Child 2');
    });

    it('onIssueChange replaces the matching issue by ref', () => {
        const target = component.issues()[0];
        const edited: StagedIssue = { ...target, title: 'Edited title', estimatedMinutes: 30 };
        component.onIssueChange(edited);
        expect(component.issues()[0].title).toBe('Edited title');
        expect(component.issues()[0].estimatedMinutes).toBe(30);
        // sibling untouched
        expect(component.issues()[1].title).toBe('Child 2');
    });

    it('onAccept emits edited values as ProposedIssue[] without the ref', () => {
        const target = component.issues()[0];
        component.onIssueChange({ ...target, title: 'Edited', estimatedMinutes: 15 });

        let emitted: ProposedIssue[] | undefined;
        component.accept.subscribe((v: ProposedIssue[]) => (emitted = v));
        component.onAccept();

        expect(emitted?.length).toBe(2);
        expect(emitted?.[0].title).toBe('Edited');
        expect(emitted?.[0].estimatedMinutes).toBe(15);
        expect((emitted?.[0] as Record<string, unknown>)['ref']).toBeUndefined();
    });

    it('onCancel emits cancelled', () => {
        let emitted = false;
        component.cancelled.subscribe(() => (emitted = true));
        component.onCancel();
        expect(emitted).toBe(true);
    });
});
