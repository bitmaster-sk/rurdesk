import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, Resolve, RouterStateSnapshot } from '@angular/router';
import { Observable, catchError, map, of, tap } from 'rxjs';
import { ProjectStore } from './project.store';
import { AclStore } from './store/acl.store';
import { ProjectMemberApi } from './api/project-member.api.service';

@Injectable({ providedIn: 'root' })
export class ProjectResolver implements Resolve<void> {
    private readonly projectStore = inject(ProjectStore);
    private readonly aclStore = inject(AclStore);
    private readonly memberApi = inject(ProjectMemberApi);

    // resolve() MUST return an Observable so Angular waits for it to complete
    // before rendering the child component. Returning void causes a race condition
    // where components render before AclStore is populated.
    //
    // projectStore.load() is fire-and-forget (returns void, subscribes internally),
    // so the project data races with route activation. The role call is the one we
    // wait on because AclStore must be populated before any child component reads it.
    public resolve(route: ActivatedRouteSnapshot, _state: RouterStateSnapshot): Observable<void> {
        const idProject = Number(route.paramMap.get('idProject'));
        this.projectStore.load(idProject);
        return this.memberApi.getUserRole(idProject).pipe(
            tap(res => this.aclStore.setRole(res.role)),
            catchError(() => {
                this.aclStore.setRole(null);
                return of(null);
            }),
            map(() => undefined)
        );
    }
}
