import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
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
    protected readonly aclStore = inject(AclStore);

    protected readonly project = toSignal(this.projectStore.project$);
}
