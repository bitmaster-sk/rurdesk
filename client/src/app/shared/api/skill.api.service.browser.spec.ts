import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { SkillApi } from './skill.api.service';

describe('SkillApi', () => {
    let api: SkillApi;
    let http: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });
        api = TestBed.inject(SkillApi);
        http = TestBed.inject(HttpTestingController);
    });

    afterEach(() => http.verify());

    it('reads the catalog from the authenticated (non-admin) route', () => {
        api.load$().subscribe();

        const request = http.expectOne('/api/private/skills');
        expect(request.request.method).toBe('GET');
        request.flush([]);
    });

    it('creates a skill with the whole body', () => {
        api.create$({ name: 'n', description: 'd', content: 'c' }).subscribe();

        const request = http.expectOne('/api/private/admin/skills');
        expect(request.request.method).toBe('POST');
        expect(request.request.body).toEqual({ name: 'n', description: 'd', content: 'c' });
        request.flush({});
    });

    it('patches only the fields it is given', () => {
        api.update$(7, { content: 'c2' }).subscribe();

        const request = http.expectOne('/api/private/admin/skills/7');
        expect(request.request.method).toBe('PATCH');
        expect(request.request.body).toEqual({ content: 'c2' });
        request.flush({});
    });

    it('deletes and restores a skill', () => {
        api.delete$(7).subscribe();
        const deleteRequest = http.expectOne('/api/private/admin/skills/7');
        expect(deleteRequest.request.method).toBe('DELETE');
        deleteRequest.flush(null);

        api.restore$(7).subscribe();
        const restoreRequest = http.expectOne('/api/private/admin/skills/7/restore');
        expect(restoreRequest.request.method).toBe('POST');
        restoreRequest.flush({});
    });
});
