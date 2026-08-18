import { Injector, runInInjectionContext } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';
import { IssueBulkApi } from './issue-bulk.api.service';
import { BulkEditIssueEntry } from '../model/bulk-edit-issues.model';

describe('IssueBulkApi', () => {
    let patch: ReturnType<typeof vi.fn>;
    let service: IssueBulkApi;

    beforeEach(() => {
        patch = vi.fn().mockReturnValue(of([]));
        const injector = Injector.create({
            providers: [{ provide: HttpClient, useValue: { patch } }]
        });
        service = runInInjectionContext(injector, () => new IssueBulkApi());
    });

    it('update$ sends PATCH to the batch endpoint with the entries wrapped in { issues }', () => {
        const entries: BulkEditIssueEntry[] = [
            { idIssuePublic: 42, scheduledAt: '2026-04-15T09:00:00Z' },
            { idIssuePublic: 43, estimated: 7200 }
        ];

        service.update$(1, entries).subscribe();

        expect(patch).toHaveBeenCalledWith('/api/private/project/1/issue/batch', {
            issues: entries
        });
    });

    it('update$ maps response date strings to Date objects', () => {
        patch.mockReturnValue(
            of([
                {
                    idIssuePublic: 1,
                    createAt: '2026-04-01T00:00:00Z',
                    updateAt: '2026-04-02T00:00:00Z',
                    scheduledAt: '2026-04-03T00:00:00Z'
                }
            ])
        );

        let res: { createAt: unknown; updateAt: unknown; scheduledAt: unknown }[] = [];
        service.update$(1, []).subscribe(r => (res = r as typeof res));

        expect(res[0].createAt).toBeInstanceOf(Date);
        expect(res[0].updateAt).toBeInstanceOf(Date);
        expect(res[0].scheduledAt).toBeInstanceOf(Date);
    });
});
