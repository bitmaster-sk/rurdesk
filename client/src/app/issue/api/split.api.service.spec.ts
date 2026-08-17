import { of } from 'rxjs';
import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { SplitApi } from './split.api.service';
import { SplitPreviewRes, SplitAcceptRes, ProposedIssue } from '../model/split.model';

describe('SplitApi', () => {
    let post: ReturnType<typeof vi.fn>;
    let service: SplitApi;

    beforeEach(() => {
        post = vi.fn();
        const injector = Injector.create({
            providers: [{ provide: HttpClient, useValue: { post } }]
        });
        service = runInInjectionContext(injector, () => new SplitApi());
    });

    it('preview$ defaults hint to empty string when undefined', () => {
        const mockResponse: SplitPreviewRes = {
            children: [{ title: 'Child 1', description: 'desc', idSeverity: null, idState: null }]
        };
        post.mockReturnValue(of(mockResponse));

        let res: SplitPreviewRes | undefined;
        service.preview$(1, 2, undefined).subscribe(r => (res = r));

        expect(post).toHaveBeenCalledWith(
            '/api/private/project/1/issue/2/split',
            { hint: '' },
            expect.objectContaining({ context: expect.anything() })
        );
        expect(res).toEqual(mockResponse);
    });

    it('preview$ POSTs the provided hint value', () => {
        post.mockReturnValue(of({ children: [] } as SplitPreviewRes));

        service.preview$(1, 2, 'separate frontend and backend').subscribe();

        expect(post).toHaveBeenCalledWith(
            '/api/private/project/1/issue/2/split',
            { hint: 'separate frontend and backend' },
            expect.objectContaining({ context: expect.anything() })
        );
    });

    it('accept$ POSTs to /api/private/project/:id/issue/:id/split/accept with children array', () => {
        const children: ProposedIssue[] = [
            { title: 'Frontend', description: 'Frontend task', idSeverity: null, idState: null },
            { title: 'Backend', description: 'Backend task', idSeverity: null, idState: null }
        ];
        const mockResponse = { children: [] } as unknown as SplitAcceptRes;
        post.mockReturnValue(of(mockResponse));

        let res: SplitAcceptRes | undefined;
        service.accept$(1, 2, children).subscribe(r => (res = r));

        expect(post).toHaveBeenCalledWith(
            '/api/private/project/1/issue/2/split/accept',
            { children },
            expect.objectContaining({ context: expect.anything() })
        );
        expect(res).toEqual(mockResponse);
    });
});
