import { describe, it, expect } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, Router, UrlTree } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';
import { projectOwnerGuard } from './project-owner.guard';
import { ProjectMemberApi } from './api/project-member.api.service';
import { Role } from '../shared/constants/role.enum';

describe('projectOwnerGuard', () => {
    const redirectTree = { toString: () => '/project/5/view' } as unknown as UrlTree;

    function run(role$: Observable<{ role: Role }>): boolean | UrlTree {
        TestBed.configureTestingModule({
            providers: [
                { provide: ProjectMemberApi, useValue: { getUserRole: () => role$ } },
                { provide: Router, useValue: { parseUrl: () => redirectTree } }
            ]
        });
        const route = {
            paramMap: { get: () => '5' }
        } as unknown as ActivatedRouteSnapshot;

        const result = TestBed.runInInjectionContext(() =>
            projectOwnerGuard(route, {} as never)
        ) as Observable<boolean | UrlTree>;

        let value = undefined as unknown as boolean | UrlTree;
        result.subscribe(v => (value = v));
        return value;
    }

    it('allows activation for an owner', () => {
        expect(run(of({ role: Role.Owner }))).toBe(true);
    });

    it('redirects a member to the project overview', () => {
        expect(run(of({ role: Role.Member }))).toBe(redirectTree);
    });

    it('redirects a viewer to the project overview', () => {
        expect(run(of({ role: Role.Viewer }))).toBe(redirectTree);
    });

    it('redirects to the project overview when the role lookup fails', () => {
        expect(run(throwError(() => new Error('boom')))).toBe(redirectTree);
    });
});
