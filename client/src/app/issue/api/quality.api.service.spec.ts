import { of } from 'rxjs';
import type { HttpClient } from '@angular/common/http';
import { QualityApi } from './quality.api.service';
import { QualityReport } from '../model/quality.model';

const mockReport: QualityReport = {
    score: 72,
    dimensions: { clarity: 80, completeness: 65, actionability: 75, scope: 90, metadata: 50 },
    problems: ['No acceptance criteria'],
    suggestions: [
        { type: 'add_section', explanation: 'Add AC', newValue: '## Acceptance Criteria' }
    ],
    checkedAt: '2026-03-20T00:00:00Z',
    fromCache: false
};

describe('QualityApi', () => {
    let post: ReturnType<typeof vi.fn>;
    let get: ReturnType<typeof vi.fn>;
    let service: QualityApi;

    beforeEach(() => {
        post = vi.fn().mockReturnValue(of(mockReport));
        get = vi.fn().mockReturnValue(of(mockReport));
        service = new QualityApi({ post, get } as unknown as HttpClient);
    });

    it('preview$ POSTs to /api/private/project/:id/quality with title and description', () => {
        let res: QualityReport | undefined;
        service.preview$(1, 'Fix login bug', 'Details here').subscribe(r => (res = r));

        expect(post).toHaveBeenCalledWith('/api/private/project/1/quality', {
            title: 'Fix login bug',
            description: 'Details here'
        });
        expect(res).toEqual(mockReport);
    });

    it('check$ POSTs to /api/private/project/:id/issue/:id/quality with title and description', () => {
        let res: QualityReport | undefined;
        service.check$(1, 42, 'Fix login bug', 'Details here').subscribe(r => (res = r));

        expect(post).toHaveBeenCalledWith('/api/private/project/1/issue/42/quality', {
            title: 'Fix login bug',
            description: 'Details here'
        });
        expect(res).toEqual(mockReport);
    });

    it('getQuality$ GETs /api/private/project/:id/issue/:id/quality', () => {
        let res: QualityReport | undefined;
        service.getQuality$(1, 42).subscribe(r => (res = r));

        expect(get).toHaveBeenCalledWith(
            '/api/private/project/1/issue/42/quality',
            expect.objectContaining({ context: expect.anything() })
        );
        expect(res).toEqual(mockReport);
    });
});
