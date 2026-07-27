import { ComponentFixture, TestBed } from '@angular/core/testing';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { By } from '@angular/platform-browser';
import { signal } from '@angular/core';
import { of } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { GitIntegrationListComponent } from './git-integration-list.component';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { AclStore } from '../../store/acl.store';
import { GitIntegrationRes, HostType } from '../../model/git-integration.model';

const makeIntegration = (idGitIntegration: number, hostType: HostType): GitIntegrationRes => ({
    idGitIntegration,
    idProject: 1,
    name: `int-${idGitIntegration}`,
    hostType,
    baseUrl: 'https://example.com',
    repoPath: 'owner/repo',
    createdAt: '',
    updatedAt: ''
});

describe('GitIntegrationListComponent', () => {
    let fixture: ComponentFixture<GitIntegrationListComponent>;
    let api: { list$: ReturnType<typeof vi.fn>; delete$: ReturnType<typeof vi.fn> };

    function setup(integrations: GitIntegrationRes[]): void {
        api.list$.mockReturnValue(of(integrations));
        fixture = TestBed.createComponent(GitIntegrationListComponent);
        fixture.componentRef.setInput('project', { idProject: 1 });
        fixture.detectChanges();
    }

    beforeEach(() => {
        api = { list$: vi.fn(), delete$: vi.fn() };

        TestBed.configureTestingModule({
            declarations: [GitIntegrationListComponent],
            imports: [HttpClientTestingModule, TranslateModule.forRoot()],
            providers: [
                { provide: GitIntegrationApi, useValue: api },
                // Read-only viewer: hides the manage column so the row never
                // pulls ui-button/tabler-icon children into this DOM-focused spec.
                { provide: AclStore, useValue: { canManageGitIntegration: signal(false) } }
            ]
        });
    });

    it('renders a brand icon whose src matches each provider', () => {
        setup([
            makeIntegration(1, HostType.GitHub),
            makeIntegration(2, HostType.GitLab),
            makeIntegration(3, HostType.Gitea)
        ]);

        const srcs = fixture.debugElement
            .queryAll(By.css('img.git-provider-icon'))
            .map(el => (el.nativeElement as HTMLImageElement).getAttribute('src'));

        expect(srcs).toEqual([
            'assets/image/icons/github.svg',
            'assets/image/icons/gitlab.svg',
            'assets/image/icons/gitea.svg'
        ]);
    });

    it('shows the human-readable provider label, not the raw host type', () => {
        setup([makeIntegration(1, HostType.Gitea)]);

        const cellText = fixture.debugElement
            .queryAll(By.css('tbody td'))[1]
            .nativeElement.textContent.trim();

        expect(cellText).toBe('Gitea');
    });
});
