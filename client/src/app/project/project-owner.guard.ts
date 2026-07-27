import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, CanActivateFn, Router, UrlTree } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { Role } from '../shared/constants/role.enum';
import { ProjectMemberApi } from './api/project-member.api.service';

/**
 * Blocks the project settings page for anyone below owner. This is UX only — the
 * server enforces every mutation (project update, git-integration CRUD) via its own
 * ACL checks. Guards run BEFORE route resolvers, so on a hard load AclStore is not
 * populated yet; the guard fetches the role itself and redirects non-owners to the
 * project overview instead of hanging on the (not-yet-run) resolver.
 */
export const projectOwnerGuard: CanActivateFn = (
    route: ActivatedRouteSnapshot
): Observable<boolean | UrlTree> => {
    const router = inject(Router);
    const memberApi = inject(ProjectMemberApi);
    const idProject = Number(route.paramMap.get('idProject'));
    const redirect = router.parseUrl(`/project/${idProject}/view`);

    return memberApi.getUserRole(idProject).pipe(
        map(({ role }) => (role === Role.Owner ? true : redirect)),
        catchError(() => of(redirect))
    );
};
