import { Injector, runInInjectionContext, ɵSIGNAL } from '@angular/core';
import { FormBuilder, NonNullableFormBuilder } from '@angular/forms';
import { of } from 'rxjs';
import { GitIntegrationSettingsComponent } from './git-integration-settings.component';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { GitIntegrationRes, HostType } from '../../model/git-integration.model';
import { Project } from '../../model/project.model';

const PROJECT: Project = { idProject: 1, name: 'P', color: '#000' };

const INTEGRATION: GitIntegrationRes = {
    idGitIntegration: 42,
    idProject: 1,
    name: 'Old',
    hostType: HostType.GitLab,
    baseUrl: 'https://gitlab.com',
    repoPath: 'team/repo',
    createdAt: '',
    updatedAt: ''
};

// Input signals are read-only functions. In a non-TestBed spec we set them
// by accessing the internal signal node and calling its applyValueToInputSignal.
function setInput(inputSignal: unknown, value: unknown): void {
    const node = (
        inputSignal as Record<
            symbol,
            { applyValueToInputSignal: (node: unknown, value: unknown) => void }
        >
    )[ɵSIGNAL];
    node.applyValueToInputSignal(node, value);
}

function createComponent(opts: {
    project?: Project;
    integration?: GitIntegrationRes | null;
    gitIntegrationApi: { create$: ReturnType<typeof vi.fn>; update$: ReturnType<typeof vi.fn> };
}): GitIntegrationSettingsComponent {
    const injector = Injector.create({
        providers: [
            { provide: NonNullableFormBuilder, useValue: new FormBuilder().nonNullable },
            { provide: GitIntegrationApi, useValue: opts.gitIntegrationApi }
        ]
    });
    const component = runInInjectionContext(injector, () => new GitIntegrationSettingsComponent());
    setInput(component.project, opts.project ?? PROJECT);
    setInput(component.integration, opts.integration ?? null);
    component.ngOnInit();
    return component;
}

describe('GitIntegrationSettingsComponent', () => {
    let gitIntegrationApi: {
        create$: ReturnType<typeof vi.fn>;
        update$: ReturnType<typeof vi.fn>;
    };

    beforeEach(() => {
        gitIntegrationApi = {
            create$: vi.fn().mockReturnValue(of(INTEGRATION)),
            update$: vi.fn().mockReturnValue(of(INTEGRATION))
        };
    });

    it('create mode: calls create$ with all five fields including accessToken', () => {
        const component = createComponent({ gitIntegrationApi });
        component.form.setValue({
            name: 'New',
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });

        component.onSave();

        expect(gitIntegrationApi.create$).toHaveBeenCalledWith(1, {
            name: 'New',
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });
    });

    it('update mode with empty token: omits accessToken from update$ payload', () => {
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });
        component.form.setValue({
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo',
            accessToken: ''
        });

        component.onSave();

        expect(gitIntegrationApi.update$).toHaveBeenCalledWith(1, 42, {
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo'
        });
        // accessToken key must NOT be present on the request object.
        const callArg = gitIntegrationApi.update$.mock.calls[0][2];
        expect(callArg).not.toHaveProperty('accessToken');
    });

    it('update mode with new token: includes accessToken in update$ payload', () => {
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });
        component.form.setValue({
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo',
            accessToken: 'newtoken'
        });

        component.onSave();

        expect(gitIntegrationApi.update$).toHaveBeenCalledWith(1, 42, {
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo',
            accessToken: 'newtoken'
        });
    });

    it('invalid form: does not call the API', () => {
        const component = createComponent({ gitIntegrationApi });
        // Form is invalid: name and baseUrl are empty (required validators not satisfied).
        component.form.setValue({
            name: '',
            hostType: HostType.GitHub,
            baseUrl: '',
            repoPath: '',
            accessToken: ''
        });

        component.onSave();

        expect(gitIntegrationApi.create$).not.toHaveBeenCalled();
        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
    });
});
