import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { AdminApi } from '../../api/admin.api.service';
import { VersionApi } from '../../api/version.api.service';
import { AdminSettingsComponent } from './admin-settings.component';

/**
 * The version row tells an operator which release the instance is running.
 * The template is trimmed to just that row so the assertions do not depend on
 * the settings form rendering.
 */
describe('AdminSettingsComponent — build version (browser)', () => {
    async function setup(buildInfo: { version: string; commit: string }) {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [AdminSettingsComponent],
            providers: [
                {
                    provide: AdminApi,
                    useValue: {
                        getSettings$: () =>
                            of({ tablePageSize: 50, kanbanPageSize: 20, ganttBacklogPageSize: 30 }),
                        updateSettings$: () => of({})
                    }
                },
                { provide: VersionApi, useValue: { getVersion$: () => of(buildInfo) } }
            ]
        })
            .overrideComponent(AdminSettingsComponent, {
                set: { template: `<span class="version">{{ versionLabel() }}</span>` }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AdminSettingsComponent);
        fixture.detectChanges();
        return fixture.nativeElement.querySelector('.version') as HTMLElement;
    }

    it('shows the version with a short commit', async () => {
        // The workflow stamps the full 40-char SHA; the UI shows the short form.
        const row = await setup({
            version: '1.0.0',
            commit: 'abc1234def5678901234567890abcdef12345678'
        });

        expect(row.textContent?.trim()).toBe('1.0.0 (abc1234)');
    });

    it('shows a dev build without a commit suffix', async () => {
        const row = await setup({ version: 'dev', commit: 'unknown' });

        expect(row.textContent?.trim()).toBe('dev');
    });
});
