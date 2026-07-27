import { of } from 'rxjs';
import type { HttpClient } from '@angular/common/http';
import { ProjectBuilderApi } from './project-builder.api.service';
import {
    ProjectBuilderGenerateReq,
    ProjectBuilderGenerateRes
} from '../model/project-builder.model';

describe('ProjectBuilderApi', () => {
    let post: ReturnType<typeof vi.fn>;
    let service: ProjectBuilderApi;

    beforeEach(() => {
        post = vi.fn();
        service = new ProjectBuilderApi({ post } as unknown as HttpClient);
    });

    it('generate$ POSTs the request body (description, idState, idSeverity) to the generate endpoint', () => {
        const mockResponse: ProjectBuilderGenerateRes = { summary: 'test', issues: [] };
        post.mockReturnValue(of(mockResponse));

        const req: ProjectBuilderGenerateReq = {
            description: 'Test project description that is long enough.',
            idState: 42,
            idSeverity: 7
        };

        let res: ProjectBuilderGenerateRes | undefined;
        service.generate$(1, req).subscribe(r => (res = r));

        expect(post).toHaveBeenCalledWith('/api/private/project/1/project-builder/generate', req);
        expect(res).toEqual(mockResponse);
    });

    it('accept$ POSTs an issues array to the accept endpoint', () => {
        post.mockReturnValue(of({ issues: [] }));

        service.accept$(1, []).subscribe();

        expect(post).toHaveBeenCalledWith('/api/private/project/1/project-builder/accept', {
            issues: []
        });
    });
});
