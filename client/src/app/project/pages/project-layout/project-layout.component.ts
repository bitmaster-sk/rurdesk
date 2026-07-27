import { ChangeDetectionStrategy, Component, OnDestroy, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AclStore } from '../../store/acl.store';
import { ProjectStore } from '../../project.store';
import { CommandPaletteService } from '../../../core/command/command-palette.service';

@Component({
    selector: 'app-project-layout',
    template: '<router-outlet></router-outlet>',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectLayoutComponent implements OnDestroy {
    private readonly aclStore = inject(AclStore);
    private readonly projectStore = inject(ProjectStore);
    private readonly commandPalette = inject(CommandPaletteService);

    constructor() {
        // Baseline command-palette context: the current project (issue is set by the detail page).
        this.projectStore.project$.pipe(takeUntilDestroyed()).subscribe(project =>
            this.commandPalette.setContext({
                idProject: project.idProject ?? null,
                issue: null
            })
        );
    }

    public ngOnDestroy(): void {
        this.aclStore.setRole(null);
        // project$ filters null and never re-emits on leave, so this teardown is the only reset —
        // without it the palette keeps offering the left project's nav/issues/people.
        this.commandPalette.setContext({ idProject: null, issue: null });
    }
}
