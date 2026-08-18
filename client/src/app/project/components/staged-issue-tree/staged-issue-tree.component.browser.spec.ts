import { Component, ChangeDetectionStrategy, Input, Output, EventEmitter } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { StagedIssueTreeComponent } from './staged-issue-tree.component';
import { ProjectBuilderIssue } from '../../model/project-builder.model';
import { IssueChangeEvent } from '../project-builder-step-staging/project-builder-step-staging.component';

@Component({
    selector: 'app-staged-issue-tree-node',
    template: '',
    standalone: false
})
class StagedIssueTreeNodeStub {
    @Input() public node: any;
    @Input() public states: any;
    @Input() public severities: any;
    @Input() public isLastChild: any;
    @Input() public ancestorHasMoreSiblings: any;
    @Output() public issueChange = new EventEmitter<IssueChangeEvent>();
    @Output() public deleteNode = new EventEmitter<StagedIssueNode>();
}

const makeIssue = (ref: string): ProjectBuilderIssue => ({
    ref,
    title: ref,
    description: '',
    estimatedMinutes: 0,
    idState: null,
    idSeverity: null,
    hierarchyParentRef: '',
    scheduleRelations: []
});

const makeNode = (ref: string): StagedIssueNode => ({
    data: makeIssue(ref),
    children: []
});

describe('StagedIssueTreeComponent', () => {
    let fixture: ComponentFixture<StagedIssueTreeComponent>;
    let component: StagedIssueTreeComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [StagedIssueTreeComponent, StagedIssueTreeNodeStub]
        })
            .overrideComponent(StagedIssueTreeComponent, {
                set: { changeDetection: ChangeDetectionStrategy.Default }
            })
            .compileComponents();

        fixture = TestBed.createComponent(StagedIssueTreeComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('roots', []);
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);
        fixture.detectChanges();
    });

    it('renders one app-staged-issue-tree-node per root', () => {
        fixture.componentRef.setInput('roots', [makeNode('I-1'), makeNode('I-2'), makeNode('I-3')]);
        fixture.detectChanges();
        const nodes = fixture.debugElement.queryAll(By.directive(StagedIssueTreeNodeStub));
        expect(nodes.length).toBe(3);
    });

    it('renders nothing when roots is empty', () => {
        const nodes = fixture.debugElement.queryAll(By.directive(StagedIssueTreeNodeStub));
        expect(nodes.length).toBe(0);
    });

    it('marks only the last root as isLastChild', () => {
        fixture.componentRef.setInput('roots', [makeNode('I-1'), makeNode('I-2')]);
        fixture.detectChanges();
        const stubs = fixture.debugElement
            .queryAll(By.directive(StagedIssueTreeNodeStub))
            .map(de => de.componentInstance as StagedIssueTreeNodeStub);
        expect(stubs[0].isLastChild).toBe(false);
        expect(stubs[1].isLastChild).toBe(true);
    });

    it('passes empty ancestorHasMoreSiblings to all roots', () => {
        fixture.componentRef.setInput('roots', [makeNode('I-1'), makeNode('I-2')]);
        fixture.detectChanges();
        const stubs = fixture.debugElement
            .queryAll(By.directive(StagedIssueTreeNodeStub))
            .map(de => de.componentInstance as StagedIssueTreeNodeStub);
        expect(stubs[0].ancestorHasMoreSiblings).toEqual([]);
        expect(stubs[1].ancestorHasMoreSiblings).toEqual([]);
    });

    it('forwards issueChange events from child nodes', () => {
        fixture.componentRef.setInput('roots', [makeNode('I-1')]);
        fixture.detectChanges();

        const emitted: IssueChangeEvent[] = [];
        component.issueChange.subscribe(e => emitted.push(e));

        const stub = fixture.debugElement.query(By.directive(StagedIssueTreeNodeStub))
            .componentInstance as StagedIssueTreeNodeStub;
        const event: IssueChangeEvent = { node: makeNode('I-1'), updated: makeIssue('I-1') };
        stub.issueChange.emit(event);

        expect(emitted.length).toBe(1);
        expect(emitted[0]).toBe(event);
    });

    it('forwards deleteNode events from child nodes', () => {
        fixture.componentRef.setInput('roots', [makeNode('I-1')]);
        fixture.detectChanges();

        const emitted: StagedIssueNode[] = [];
        component.deleteNode.subscribe(n => emitted.push(n));

        const stub = fixture.debugElement.query(By.directive(StagedIssueTreeNodeStub))
            .componentInstance as StagedIssueTreeNodeStub;
        const node = makeNode('I-1');
        stub.deleteNode.emit(node);

        expect(emitted[0]).toBe(node);
    });
});
