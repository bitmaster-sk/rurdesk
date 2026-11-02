import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { TagStore } from 'src/app/tag/store/tag.store';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseTagAction } from '../../constants/release-tag-action.enum';
import { ReleaseView } from '../../model/release.model';
import { ReleaseDeleteDialogComponent } from './release-delete-dialog.component';

const RELEASE: ReleaseView = {
    idRelease: 5,
    idProject: 3,
    idTag: 9,
    name: 'v1.0',
    status: ReleaseStatus.Released,
    notes: '## Fixes',
    recreateTag: true,
    openIssues: ReleaseOpenIssues.Move,
    issueCount: 2,
    createAt: '2026-10-27T10:00:00Z',
    createBy: 1,
    releasedAt: '2026-10-27T11:00:00Z',
    releasedBy: 1,
    tagName: 'v1.0',
    idRunNotes: null,
    notesRunPhase: null,
    notesRunError: null,
    isNotesPending: false
};

describe('ReleaseDeleteDialogComponent', () => {
    let api: { delete$: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        api = { delete$: vi.fn().mockReturnValue(of(RELEASE)) };
    });

    async function setup(release: ReleaseView = RELEASE) {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [ReleaseDeleteDialogComponent],
            providers: [
                { provide: ReleaseApi, useValue: api },
                {
                    provide: TagStore,
                    useValue: {
                        tagsByProject$: () =>
                            of([
                                {
                                    idTag: 9,
                                    idProject: 3,
                                    name: 'v1.0',
                                    color: '#000',
                                    orderRank: 1,
                                    issueCount: 2
                                },
                                {
                                    idTag: 10,
                                    idProject: 3,
                                    name: 'next',
                                    color: '#000',
                                    orderRank: 2,
                                    issueCount: 0
                                }
                            ])
                    }
                }
            ]
        })
            .overrideComponent(ReleaseDeleteDialogComponent, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(ReleaseDeleteDialogComponent);
        fixture.componentRef.setInput('release', release);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    it('keeps the version tag by default and sends no target', async () => {
        const dialog = await setup();

        dialog['onDelete']();

        expect(api.delete$).toHaveBeenCalledWith(3, 5, { tag: ReleaseTagAction.Keep });
    });

    it('needs another tag before the tasks can move', async () => {
        const dialog = await setup();

        dialog['form'].controls.action.setValue(ReleaseTagAction.MoveDelete);

        expect(dialog['isValid']()).toBe(false);
        expect(dialog['targets']().map(tag => tag.name)).toEqual(['next']);
        dialog['form'].controls.idTagTarget.setValue(10);
        expect(dialog['isValid']()).toBe(true);

        dialog['onDelete']();

        expect(api.delete$).toHaveBeenCalledWith(3, 5, {
            tag: ReleaseTagAction.MoveDelete,
            idTagTarget: 10
        });
        expect(dialog['visible']()).toBe(false);
    });

    it('deletes a draft without asking about its tag', async () => {
        const dialog = await setup({ ...RELEASE, status: ReleaseStatus.Draft });

        dialog['form'].controls.action.setValue(ReleaseTagAction.MoveDelete);

        expect(dialog['isValid']()).toBe(true);
        dialog['onDelete']();
        expect(api.delete$).toHaveBeenCalledWith(3, 5, {});
    });
});
