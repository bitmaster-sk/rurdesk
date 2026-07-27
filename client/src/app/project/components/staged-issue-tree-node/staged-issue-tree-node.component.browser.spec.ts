import { ComponentFixture, TestBed } from '@angular/core/testing';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { StagedIssueTreeNodeComponent } from './staged-issue-tree-node.component';
import { ProjectBuilderIssue } from '../../model/project-builder.model';
import { IssueChangeEvent } from '../project-builder-step-staging/project-builder-step-staging.component';

const mockIssue: ProjectBuilderIssue = {
    ref: 'I-1',
    title: 'Parent',
    description: '',
    estimatedMinutes: 0,
    idState: null,
    idSeverity: null,
    hierarchyParentRef: '',
    scheduleRelations: []
};

const makeNode = (ref: string, children: StagedIssueNode[] = []): StagedIssueNode => ({
    data: { ...mockIssue, ref, title: ref },
    children
});

describe('StagedIssueTreeNodeComponent', () => {
    let fixture: ComponentFixture<StagedIssueTreeNodeComponent>;
    let component: StagedIssueTreeNodeComponent;

    function setup(node: StagedIssueNode, isLastChild = true, ancestors: boolean[] = []): void {
        fixture.componentRef.setInput('node', node);
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);
        fixture.componentRef.setInput('isLastChild', isLastChild);
        fixture.componentRef.setInput('ancestorHasMoreSiblings', ancestors);
        fixture.detectChanges();
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [StagedIssueTreeNodeComponent]
        })
            // Empty template — these tests exercise computed signals and event
            // wrappers directly, not the rendered DOM (which pulls child components).
            .overrideComponent(StagedIssueTreeNodeComponent, {
                set: { template: '' }
            })
            .compileComponents();

        fixture = TestBed.createComponent(StagedIssueTreeNodeComponent);
        component = fixture.componentInstance;
    });

    describe('depth', () => {
        it('is 1 when ancestorHasMoreSiblings is empty', () => {
            setup(makeNode('I-1'), true, []);
            expect(component['depth']()).toBe(1);
        });

        it('is 3 when ancestorHasMoreSiblings has 2 entries', () => {
            setup(makeNode('I-1'), true, [true, false]);
            expect(component['depth']()).toBe(3);
        });
    });

    describe('childAncestorHasMoreSiblings', () => {
        it('appends true when current node is NOT last child', () => {
            setup(makeNode('I-1', [makeNode('I-2')]), false, []);
            expect(component['childAncestorHasMoreSiblings']()).toEqual([true]);
        });

        it('appends false when current node IS last child', () => {
            setup(makeNode('I-1', [makeNode('I-2')]), true, [false]);
            expect(component['childAncestorHasMoreSiblings']()).toEqual([false, false]);
        });

        it('preserves existing ancestor flags', () => {
            setup(makeNode('I-1', [makeNode('I-2')]), false, [true, false]);
            expect(component['childAncestorHasMoreSiblings']()).toEqual([true, false, true]);
        });
    });

    describe('event wrapping', () => {
        it('merges card issueChange onto node data, preserving hierarchy fields', () => {
            const node = makeNode('I-1');
            node.data!.hierarchyParentRef = 'P-9';
            setup(node, true, []);
            const emitted: IssueChangeEvent[] = [];
            component.issueChange.subscribe(e => emitted.push(e));

            // The shared card emits only StagedIssue fields (no hierarchy/schedule).
            component.onCardIssueChange({
                ref: 'I-1',
                title: 'Updated',
                description: '',
                estimatedMinutes: 0,
                idState: null,
                idSeverity: null
            });

            expect(emitted.length).toBe(1);
            expect(emitted[0].node).toBe(node);
            expect(emitted[0].updated.title).toBe('Updated');
            // hierarchy field came from node data, not the card payload
            expect(emitted[0].updated.hierarchyParentRef).toBe('P-9');
        });

        it('emits deleteNode with the node on card delete', () => {
            const node = makeNode('I-1');
            setup(node, true, []);
            const emitted: StagedIssueNode[] = [];
            component.deleteNode.subscribe(n => emitted.push(n));

            component.onCardDelete();

            expect(emitted.length).toBe(1);
            expect(emitted[0]).toBe(node);
        });
    });
});
