import { inject } from '@angular/core';
import { CanMatchFn } from '@angular/router';
import { map } from 'rxjs/operators';
import { ProjectService } from '../project/project.service';
import { Project } from '../project/model/project.model';

export abstract class FirstProjectGuard {
    /** True when the user owns no projects — the first-run onboarding condition. */
    public static hasNoProjects(projects: Project[]): boolean {
        return projects.length === 0;
    }

    /**
     * Matches the onboarding route ahead of UserModule only for a brand-new user
     * with zero projects. Returning false lets routing fall through to the next
     * empty-path route (the normal "My page").
     */
    public static readonly canMatch: CanMatchFn = () => {
        const projectService = inject(ProjectService);
        return projectService.loadProjects().pipe(map(FirstProjectGuard.hasNoProjects));
    };
}
