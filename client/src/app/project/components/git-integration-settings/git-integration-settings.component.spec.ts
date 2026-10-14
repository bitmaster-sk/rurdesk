import { Injector, runInInjectionContext, ɵSIGNAL } from '@angular/core';
import { FormBuilder, NonNullableFormBuilder } from '@angular/forms';
import { Subject, of, throwError } from 'rxjs';
import { GitIntegrationSettingsComponent } from './git-integration-settings.component';
import { GitIntegrationApi } from '../../api/git-integration.api.service';
import { GitAccessResult, GitIntegrationRes, HostType } from '../../model/git-integration.model';
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

interface GitIntegrationApiMock {
    insert$: ReturnType<typeof vi.fn>;
    update$: ReturnType<typeof vi.fn>;
    check$: ReturnType<typeof vi.fn>;
}

function createComponent(opts: {
    project?: Project;
    integration?: GitIntegrationRes | null;
    gitIntegrationApi: GitIntegrationApiMock;
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
    let gitIntegrationApi: GitIntegrationApiMock;

    beforeEach(() => {
        gitIntegrationApi = {
            insert$: vi.fn().mockReturnValue(of(INTEGRATION)),
            update$: vi.fn().mockReturnValue(of(INTEGRATION)),
            check$: vi.fn().mockReturnValue(of({ result: GitAccessResult.Ok }))
        };
    });

    it('create mode: calls insert$ with all five fields including accessToken', () => {
        const component = createComponent({ gitIntegrationApi });
        component['form'].setValue({
            name: 'New',
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });

        component['onSave']();

        expect(gitIntegrationApi.insert$).toHaveBeenCalledWith(1, {
            name: 'New',
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });
    });

    it('update mode with empty token: omits accessToken from update$ payload', () => {
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });
        component['form'].setValue({
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo',
            accessToken: ''
        });

        component['onSave']();

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
        component['form'].setValue({
            name: 'Updated',
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo',
            accessToken: 'newtoken'
        });

        component['onSave']();

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
        component['form'].setValue({
            name: '',
            hostType: HostType.GitHub,
            baseUrl: '',
            repoPath: '',
            accessToken: ''
        });

        component['onSave']();

        expect(gitIntegrationApi.insert$).not.toHaveBeenCalled();
        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
    });

    it('create mode: prefills the GitHub base URL', () => {
        const component = createComponent({ gitIntegrationApi });

        expect(component['form'].controls.baseUrl.value).toBe('https://github.com');
    });

    it('switching host type replaces a provider default base URL', () => {
        const component = createComponent({ gitIntegrationApi });

        component['form'].controls.hostType.setValue(HostType.GitLab);

        expect(component['form'].controls.baseUrl.value).toBe('https://gitlab.com');
    });

    it('switching host type keeps a custom base URL', () => {
        const component = createComponent({ gitIntegrationApi });
        component['form'].controls.baseUrl.setValue('https://git.example.com');

        component['form'].controls.hostType.setValue(HostType.Gitea);

        expect(component['form'].controls.baseUrl.value).toBe('https://git.example.com');
    });

    it('update mode: keeps the stored base URL', () => {
        const component = createComponent({
            gitIntegrationApi,
            integration: { ...INTEGRATION, baseUrl: 'https://gitlab.example.com' }
        });

        expect(component['form'].controls.baseUrl.value).toBe('https://gitlab.example.com');
    });

    it('create mode check: sends the typed token', () => {
        const component = createComponent({ gitIntegrationApi });
        component['form'].setValue({
            name: 'New',
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });

        component['onCheck']();

        expect(gitIntegrationApi.check$).toHaveBeenCalledWith(1, {
            hostType: HostType.GitHub,
            baseUrl: 'https://github.com',
            repoPath: 'owner/repo',
            accessToken: 'tok123'
        });
        expect(component['checkResult']()).toEqual({ result: GitAccessResult.Ok });
    });

    it('update mode check with empty token: asks the backend to use the stored token', () => {
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });

        component['onCheck']();

        expect(gitIntegrationApi.check$).toHaveBeenCalledWith(1, {
            idGitIntegration: 42,
            hostType: HostType.GitLab,
            baseUrl: 'https://gitlab.com',
            repoPath: 'team/repo'
        });
    });

    it('failed check on save: does not save until the user saves again', () => {
        gitIntegrationApi.check$.mockReturnValue(of({ result: GitAccessResult.InvalidToken }));
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });

        component['onSave']();

        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
        expect(component['isCheckFailed']()).toBe(true);

        component['onSave']();

        expect(gitIntegrationApi.check$).toHaveBeenCalledTimes(1);
        expect(gitIntegrationApi.update$).toHaveBeenCalledTimes(1);
    });

    it('editing the form after a failed check: checks again on save', () => {
        gitIntegrationApi.check$.mockReturnValue(of({ result: GitAccessResult.RepoNotFound }));
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });
        component['onSave']();

        component['form'].controls.repoPath.setValue('team/other');

        expect(component['checkResult']()).toBeNull();
        component['onSave']();
        expect(gitIntegrationApi.check$).toHaveBeenCalledTimes(2);
        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
    });

    it('check request error: reports the host as unreachable and does not save', () => {
        gitIntegrationApi.check$.mockReturnValue(throwError(() => new Error('boom')));
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });

        component['onSave']();

        expect(component['checkResult']()).toEqual({ result: GitAccessResult.Unreachable });
        expect(component['isChecking']()).toBe(false);
        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
    });

    it('editing the form while a check runs: drops the stale result and does not save', () => {
        const response = new Subject<{ result: GitAccessResult }>();
        gitIntegrationApi.check$.mockReturnValue(response);
        const component = createComponent({ gitIntegrationApi, integration: INTEGRATION });

        component['onSave']();
        component['form'].controls.repoPath.setValue('team/other');
        response.next({ result: GitAccessResult.Ok });
        response.complete();

        expect(component['checkResult']()).toBeNull();
        expect(component['isChecking']()).toBe(false);
        expect(gitIntegrationApi.update$).not.toHaveBeenCalled();
    });
});
