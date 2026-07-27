import { inject, Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, Resolve, RouterStateSnapshot } from '@angular/router';
import { Observable } from 'rxjs';
import { ProjectMemberStore } from './project-member.store';

@Injectable({ providedIn: 'root' })
export class ProjectMemberResolver implements Resolve<void> {
    private readonly projectMemberStore = inject(ProjectMemberStore);

    public resolve(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot
    ): Observable<any> | Promise<any> | any {
        const idProject = Number(route.paramMap.get('idProject'));
        this.projectMemberStore.load(idProject);
    }
}
