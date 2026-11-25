import { ChangeDetectionStrategy, Component, OnInit, inject } from '@angular/core';
import { ProjectContextStore } from '../../store/project-context.store';

/** The one shell around every signed-in page: side menu on the left, the routed page beside it. */
@Component({
    selector: 'app-shell-layout',
    templateUrl: './shell-layout.component.html',
    styleUrls: ['./shell-layout.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ShellLayoutComponent implements OnInit {
    private readonly projectContext = inject(ProjectContextStore);

    public ngOnInit(): void {
        this.projectContext.load();
    }
}
