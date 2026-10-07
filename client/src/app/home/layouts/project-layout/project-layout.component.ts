import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRouteSnapshot, NavigationEnd, Router } from '@angular/router';
import { filter, map } from 'rxjs';
import { ProjectStore } from 'src/app/project/project.store';
import { AclStore } from 'src/app/project/store/acl.store';

@Component({
    selector: 'app-project-layout',
    templateUrl: './project-layout.component.html',
    styleUrls: ['./project-layout.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectLayoutComponent {
    private readonly projectStore = inject(ProjectStore);
    private readonly router = inject(Router);
    protected readonly aclStore = inject(AclStore);

    protected readonly project = toSignal(this.projectStore.project$);

    protected readonly isResting = signal(false);

    protected readonly isCollapsed = toSignal(
        this.router.events.pipe(
            filter(event => event instanceof NavigationEnd),
            map(() => this.readCollapsed())
        ),
        { initialValue: this.readCollapsed() }
    );

    protected onMenuClick(): void {
        this.isResting.set(true);
    }

    protected onMenuLeave(): void {
        this.isResting.set(false);
    }

    private readCollapsed(): boolean {
        let route: ActivatedRouteSnapshot | null = this.router.routerState.snapshot.root;
        while (route) {
            if (route.data['collapsedMenu'] === true) {
                return true;
            }
            route = route.firstChild;
        }
        return false;
    }
}
