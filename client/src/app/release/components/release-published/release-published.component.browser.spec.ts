import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseView } from '../../model/release.model';
import { ReleasePublishedComponent } from './release-published.component';

const RELEASED: ReleaseView = {
    idRelease: 5,
    idProject: 3,
    idTag: 9,
    name: 'v1.1',
    status: ReleaseStatus.Released,
    notes: '## Fixed',
    recreateTag: true,
    openIssues: ReleaseOpenIssues.Move,
    issueCount: 1,
    createAt: '2026-10-27T10:00:00Z',
    createBy: 1,
    releasedAt: '2026-10-28T10:00:00Z',
    releasedBy: 1,
    tagName: 'v1.1',
    idRunNotes: null,
    notesRunPhase: null,
    notesRunError: null,
    isNotesPending: false
};

describe('ReleasePublishedComponent', () => {
    async function setup() {
        await TestBed.configureTestingModule({ declarations: [ReleasePublishedComponent] })
            .overrideComponent(ReleasePublishedComponent, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(ReleasePublishedComponent);
        fixture.componentRef.setInput('release', RELEASED);
        fixture.componentRef.setInput('canEdit', true);
        fixture.detectChanges();
        const saved: string[] = [];
        fixture.componentInstance.notesSaved.subscribe(notes => saved.push(notes));
        return { component: fixture.componentInstance, saved };
    }

    it('starts editing from the released notes', async () => {
        const { component } = await setup();
        let started = false;
        component.editStarted.subscribe(() => (started = true));

        component['onEdit']();

        expect(started).toBe(true);
        expect(component['notes']()).toBe('## Fixed');
    });

    it('saves only changed notes that have some text', async () => {
        const { component, saved } = await setup();
        component['onEdit']();

        component['onSave']();
        component['onNotesChange']('   ');
        component['onSave']();
        component['onNotesChange']('## Fixed\n\n- Login (#4)');
        component['onSave']();

        expect(saved).toEqual(['## Fixed\n\n- Login (#4)']);
    });
});
