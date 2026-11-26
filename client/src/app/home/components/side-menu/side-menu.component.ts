import { ChangeDetectionStrategy, Component, OnDestroy, OnInit, inject } from '@angular/core';
import { Router, isActive } from '@angular/router';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { HotkeyService } from 'src/app/core/command/hotkey.service';
import { TrackerControlVariant } from 'src/app/shared/tracker/constants/tracker-control-variant.enum';
import { ProjectContextStore } from '../../store/project-context.store';
import { SideMenuStore } from '../../store/side-menu.store';

@Component({
    selector: 'app-side-menu',
    templateUrl: './side-menu.component.html',
    styleUrls: ['./side-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SideMenuComponent implements OnInit, OnDestroy {
    private readonly router = inject(Router);
    private readonly projectContext = inject(ProjectContextStore);
    private readonly palette = inject(CommandPaletteService);
    private readonly hotkeys = inject(HotkeyService);
    private readonly sideMenuStore = inject(SideMenuStore);

    protected readonly TrackerControlVariant = TrackerControlVariant;

    protected readonly idProject = this.projectContext.idProject;
    protected readonly canUpdateProject = this.projectContext.canUpdateProject;

    protected readonly isOnMessages = isActive('/message', this.router);
    protected readonly isOnAgentChat = isActive('/agent-chat', this.router);

    protected readonly isCollapsed = this.sideMenuStore.isCollapsed;

    public ngOnInit(): void {
        this.hotkeys.registerRailToggle({ toggle: () => this.sideMenuStore.toggle() });
    }

    public ngOnDestroy(): void {
        this.hotkeys.registerRailToggle(null);
    }

    protected onSearch(): void {
        this.palette.open();
    }

    protected onToggle(): void {
        this.sideMenuStore.toggle();
    }
}
