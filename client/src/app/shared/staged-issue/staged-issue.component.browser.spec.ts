import { ComponentFixture, TestBed } from '@angular/core/testing';
import { StagedIssueComponent } from './staged-issue.component';
import { StagedIssue } from './staged-issue.model';

const mockIssue: StagedIssue = {
    ref: 'I-1',
    title: 'Test issue',
    description: 'A description',
    estimatedMinutes: 120,
    idState: null,
    idSeverity: null
};

describe('StagedIssueComponent', () => {
    let fixture: ComponentFixture<StagedIssueComponent>;
    let component: StagedIssueComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [StagedIssueComponent]
        })
            // Override the template to empty — these tests exercise the component's
            // logic (methods, signals) directly, not the DOM. Rendering the real
            // template would pull in child components (app-severity-dropdown, p-select)
            // and ngModel value accessors that are irrelevant here.
            .overrideComponent(StagedIssueComponent, {
                set: { template: '' }
            })
            .compileComponents();

        fixture = TestBed.createComponent(StagedIssueComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('issue', mockIssue);
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);
        fixture.detectChanges();
    });

    it('emits issueChange with updated title', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onTitleChange('New title');
        expect(emitted.length).toBe(1);
        expect(emitted[0].title).toBe('New title');
    });

    it('emits issueChange with updated description', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onDescriptionChange('New desc');
        expect(emitted[0].description).toBe('New desc');
    });

    it('parses duration text to minutes when emitting estimate', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onEstimateChange('1h 10m');
        expect(emitted[0].estimatedMinutes).toBe(70);
    });

    it('treats a bare number as hours', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onEstimateChange('3');
        expect(emitted[0].estimatedMinutes).toBe(180);
    });

    it('emits 0 minutes when estimate is cleared', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onEstimateChange('');
        expect(emitted[0].estimatedMinutes).toBe(0);
    });

    it('formats estimatedMinutes into duration text for display', () => {
        // mockIssue has estimatedMinutes = 120
        expect(component.estimateText()).toBe('2h');
    });

    it('emits issueChange with updated severity', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onSeverityChange(2);
        expect(emitted[0].idSeverity).toBe(2);
    });

    it('emits issueChange with updated state', () => {
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onStateChange(5);
        expect(emitted[0].idState).toBe(5);
    });

    it('emits delete on delete click', () => {
        let emittedCount = 0;
        component.delete.subscribe(() => emittedCount++);
        component.onDeleteClick();
        expect(emittedCount).toBe(1);
    });

    it('resets editIssue when issue input changes', () => {
        const updated = { ...mockIssue, title: 'Changed externally' };
        fixture.componentRef.setInput('issue', updated);
        fixture.detectChanges();
        expect(component.editIssue().title).toBe('Changed externally');
    });

    it('does not emit stale data after issue input resets', () => {
        component.onTitleChange('local edit');
        const updated = { ...mockIssue, title: 'Server reset' };
        fixture.componentRef.setInput('issue', updated);
        fixture.detectChanges();
        const emitted: StagedIssue[] = [];
        component.issueChange.subscribe(v => emitted.push(v));
        component.onDescriptionChange('x');
        expect(emitted[0].title).toBe('Server reset');
    });
});
