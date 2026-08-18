// @vitest-environment jsdom
import { HttpClient, type HttpParams } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { IssueService } from './issue.service';
import { IssuesFilter } from './components/filter/issue-filter.entity';
import { Issue } from './model/issue.model';
import { IssuesPage } from './model/issues-page.model';

function build(getReturn: unknown) {
    const get = vi.fn().mockReturnValue(of(getReturn));
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: { get } }] });
    const service = runInInjectionContext(injector, () => new IssueService());
    return { service, get };
}

const page = (
    items: unknown[],
    nextCursor: string | null = null,
    total = items.length
): IssuesPage => ({ items, nextCursor, total }) as unknown as IssuesPage;

const baseFilter = {
    idProject: 1,
    idsSeverity: [],
    idsState: [],
    idsAssignedTo: [],
    stateUnset: false,
    severityUnset: false,
    assignedToUnset: false
} as unknown as IssuesFilter;

describe('IssueService.loadIssues', () => {
    it('hits the project issue endpoint and serializes list params', () => {
        const { service, get } = build(page([]));
        service
            .loadIssues({
                ...baseFilter,
                idsSeverity: [1, 2],
                orderColumn: 'updateAt'
            } as IssuesFilter)
            .subscribe();

        const [url, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        expect(url).toBe('/api/private/project/1/issue');
        expect(options.params.get('idsSeverity')).toBe('1,2');
        expect(options.params.get('orderColumn')).toBe('updateAt');
        expect(options.params.get('stateUnset')).toBe('false');
    });

    it('unwraps the envelope items and normalizes date fields', () => {
        const { service } = build(
            page([
                {
                    createAt: '2026-01-01T00:00:00Z',
                    updateAt: '2026-01-02T00:00:00Z',
                    scheduledAt: null
                }
            ])
        );
        let result: Issue[] = [];
        service.loadIssues(baseFilter).subscribe(r => (result = r));

        expect(result.length).toBe(1);
        expect(result[0].createAt).toBeInstanceOf(Date);
        expect(result[0].updateAt).toBeInstanceOf(Date);
        expect(result[0].scheduledAt).toBeNull();
    });
});

describe('IssueService.loadIssuesPage$', () => {
    it('forwards limit + cursor and keeps nextCursor/total', () => {
        const { service, get } = build(page([], 'abc', 42));
        let res!: IssuesPage;
        service.loadIssuesPage$(baseFilter, 50, 'cur').subscribe(r => (res = r));

        expect(res.nextCursor).toBe('abc');
        expect(res.total).toBe(42);
        const [, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        expect(options.params.get('limit')).toBe('50');
        expect(options.params.get('cursor')).toBe('cur');
    });

    it('omits the cursor param on the first page', () => {
        const { service, get } = build(page([]));
        service.loadIssuesPage$(baseFilter, 50, null).subscribe();
        const [, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        expect(options.params.has('cursor')).toBe(false);
    });
});

describe('IssueService.loadIssuesGrouped$', () => {
    it('sends groupBy + limit and maps group items', () => {
        const { service, get } = build({
            groups: [
                {
                    key: { idState: 1 },
                    items: [{ updateAt: '2026-01-01T00:00:00Z' }],
                    total: 3,
                    nextCursor: 'x'
                }
            ]
        });
        let groups: { key: unknown; items: Issue[] }[] = [];
        service.loadIssuesGrouped$(baseFilter, 'state', 20).subscribe(r => (groups = r.groups));

        const [, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        expect(options.params.get('groupBy')).toBe('state');
        expect(options.params.get('limit')).toBe('20');
        expect(groups[0].items[0].updateAt).toBeInstanceOf(Date);
    });
});

describe('IssueService.toIssue', () => {
    it('converts scheduledAt when present', () => {
        const { service } = build(page([]));
        const out = service.toIssue({
            createAt: '2026-01-01T00:00:00Z',
            updateAt: '2026-01-01T00:00:00Z',
            scheduledAt: '2026-03-03T00:00:00Z'
        } as unknown as Issue);
        expect(out.scheduledAt).toBeInstanceOf(Date);
    });
});

describe('IssueService date params', () => {
    const call = (filter: Partial<IssuesFilter>) => {
        const { service, get } = build(page([]));
        service.loadIssues({ ...baseFilter, ...filter } as IssuesFilter).subscribe();
        const [, options] = get.mock.calls[0] as [string, { params: HttpParams }];
        return options.params;
    };

    it('sends a rolling window verbatim — the API owns the grammar', () => {
        const params = call({ updateAtWithin: '1d8h6m' });
        expect(params.get('updateAtWithin')).toBe('1d8h6m');
    });

    it('sends absolute bounds when no window is set', () => {
        const params = call({
            createAtFrom: new Date('2026-01-01T00:00:00Z'),
            createAtTo: new Date('2026-01-31T00:00:00Z')
        });
        expect(params.get('createAtFrom')).toBe('2026-01-01T00:00:00.000Z');
        expect(params.get('createAtTo')).toBe('2026-01-31T00:00:00.000Z');
        expect(params.has('createAtWithin')).toBe(false);
    });

    it('drops the absolute bounds when a window is set for the same field', () => {
        const params = call({
            createAtWithin: '30d',
            createAtFrom: new Date('2026-01-01T00:00:00Z'),
            createAtTo: new Date('2026-01-31T00:00:00Z')
        });
        expect(params.get('createAtWithin')).toBe('30d');
        expect(params.has('createAtFrom')).toBe(false);
        expect(params.has('createAtTo')).toBe(false);
    });

    it('keeps the two fields independent', () => {
        const params = call({
            createAtWithin: '7d',
            updateAtFrom: new Date('2026-02-02T00:00:00Z')
        });
        expect(params.get('createAtWithin')).toBe('7d');
        expect(params.get('updateAtFrom')).toBe('2026-02-02T00:00:00.000Z');
        expect(params.has('updateAtWithin')).toBe(false);
    });
});
