import { TestBed } from '@angular/core/testing';
import { RouterModule } from '@angular/router';
import { of } from 'rxjs';
import { VersionApi } from '../../api/version.api.service';
import { AdminSettingsLayoutComponent } from './admin-settings-layout.component';

/**
 * The version row tells an operator which release the instance is running.
 * The template is trimmed to just that row so the assertions do not depend on
 * the navigation rendering.
 */
describe('AdminSettingsLayoutComponent — build version (browser)', () => {
    async function setup(buildInfo: { version: string; commit: string }) {
        await TestBed.configureTestingModule({
            declarations: [AdminSettingsLayoutComponent],
            providers: [{ provide: VersionApi, useValue: { load$: () => of(buildInfo) } }]
        })
            .overrideComponent(AdminSettingsLayoutComponent, {
                set: { template: `<span class="version">{{ versionLabel() }}</span>` }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AdminSettingsLayoutComponent);
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

describe('AdminSettingsLayoutComponent — navigation (browser)', () => {
    it('links every settings section', async () => {
        await TestBed.configureTestingModule({
            imports: [RouterModule.forRoot([])],
            declarations: [AdminSettingsLayoutComponent],
            providers: [
                {
                    provide: VersionApi,
                    useValue: { load$: () => of({ version: 'dev', commit: 'unknown' }) }
                }
            ]
        })
            .overrideComponent(AdminSettingsLayoutComponent, {
                set: {
                    template: `@for (section of sections; track section.path) {<a class="nav" [routerLink]="section.path">{{ section.path }}</a>}`
                }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsLayoutComponent);
        fixture.detectChanges();

        const links = Array.from(
            fixture.nativeElement.querySelectorAll('a.nav') as NodeListOf<HTMLAnchorElement>
        ).map(link => link.getAttribute('href'));
        expect(links).toEqual(['/general', '/agents', '/sso']);
    });
});
