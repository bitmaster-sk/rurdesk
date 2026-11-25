import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRouteSnapshot, NavigationEnd, Router } from '@angular/router';
import { filter, map } from 'rxjs';
import { AclStore } from 'src/app/project/store/acl.store';
import { ProjectContextStore } from '../../store/project-context.store';

@Component({
    selector: 'app-side-menu',
    templateUrl: './side-menu.component.html',
    styleUrls: ['./side-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SideMenuComponent {
    private readonly router = inject(Router);
    private readonly projectContext = inject(ProjectContextStore);
    protected readonly aclStore = inject(AclStore);

    protected readonly idProject = this.projectContext.idProject;

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
