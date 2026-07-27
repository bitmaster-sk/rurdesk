import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ProjectBuilderStepStagingComponent } from './project-builder-step-staging.component';
import { TranslateModule } from '@ngx-translate/core';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { ProjectBuilderIssue } from '../../model/project-builder.model';

@Component({ selector: 'app-staged-issue-tree', template: '', standalone: true })
class StagedIssueTreeStub {
    public readonly roots = input<unknown[]>([]);
    public readonly states = input<unknown[]>([]);
    public readonly severities = input<unknown[]>([]);
    public readonly issueChange = output<unknown>();
    public readonly deleteNode = output<unknown>();
}

describe('ProjectBuilderStepStagingComponent', () => {
    let fixture: ComponentFixture<ProjectBuilderStepStagingComponent>;
    let component: ProjectBuilderStepStagingComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [ProjectBuilderStepStagingComponent],
            imports: [TranslateModule.forRoot(), TablerIconStub, UiButtonStub, StagedIssueTreeStub]
        })
            .overrideComponent(ProjectBuilderStepStagingComponent, {
                set: { changeDetection: ChangeDetectionStrategy.Default }
            })
            .compileComponents();

        fixture = TestBed.createComponent(ProjectBuilderStepStagingComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('stagedIssues', [] as StagedIssueNode[]);
        fixture.componentRef.setInput('summary', '');
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);
        fixture.componentRef.setInput('isAccepting', false);
        fixture.componentRef.setInput('isRestoreBannerVisible', false);
        fixture.componentRef.setInput('restoredFrom', '');
        fixture.componentRef.setInput('flatCount', 0);
    });

    it('emits accept when onAccept is called', () => {
        let emittedCount = 0;
        component.accept.subscribe(() => emittedCount++);
        component.onAccept();
        expect(emittedCount).toBe(1);
    });

    it('emits back when onBack is called', () => {
        let emittedCount = 0;
        component.back.subscribe(() => emittedCount++);
        component.onBack();
        expect(emittedCount).toBe(1);
    });

    it('emits regenerate when onRegenerate is called', () => {
        let emittedCount = 0;
        component.regenerate.subscribe(() => emittedCount++);
        component.onRegenerate();
        expect(emittedCount).toBe(1);
    });
});
