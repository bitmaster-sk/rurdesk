import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { VersionApi } from '../../api/version.api.service';
import { BuildInfo } from '../../model/build-info.model';
import { ADMIN_SETTINGS_SECTIONS } from '../../constants/admin-settings-sections';

@Component({
    selector: 'app-admin-settings-layout',
    templateUrl: './admin-settings-layout.component.html',
    styleUrls: ['./admin-settings-layout.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsLayoutComponent {
    private readonly versionApi = inject(VersionApi);

    protected readonly sections = ADMIN_SETTINGS_SECTIONS;

    private readonly buildInfo = signal<BuildInfo | null>(null);

    /** e.g. "1.0.0 (abc1234)". An unstamped local build has no commit to show. */
    protected readonly versionLabel = computed(() => {
        const info = this.buildInfo();
        if (!info) {
            return '';
        }
        if (info.commit === 'unknown') {
            return info.version;
        }
        return `${info.version} (${info.commit.slice(0, 7)})`;
    });

    public constructor() {
        this.versionApi
            .load$()
            .pipe(takeUntilDestroyed())
            .subscribe(info => this.buildInfo.set(info));
    }
}
