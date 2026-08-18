import { inject, Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, Resolve } from '@angular/router';
import { ProjectMemberStore } from './project-member.store';

@Injectable({ providedIn: 'root' })
export class ProjectMemberResolver implements Resolve<void> {
    private readonly projectMemberStore = inject(ProjectMemberStore);

    public resolve(route: ActivatedRouteSnapshot): void {
        const idProject = Number(route.paramMap.get('idProject'));
        this.projectMemberStore.load(idProject);
    }
}
