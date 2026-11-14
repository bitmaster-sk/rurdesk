import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconArrowDownLeft,
    IconArrowRight,
    IconArrowUpRight,
    IconBook,
    IconChevronRight,
    IconCircle,
    IconCircleCheck,
    IconCopy,
    IconInfoCircle,
    IconListCheck,
    IconLock,
    IconPlugConnected,
    IconShield,
    IconShieldCheck,
    IconTrash,
    IconUsersGroup,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { Observable, of, throwError } from 'rxjs';
import { UiModule } from 'src/app/ui/ui.module';
import { ssoState } from 'src/testing/sso-state.fixture';
import { AdminSsoApi } from '../../../api/admin-sso.api.service';
import { SsoConnectionFormComponent } from '../../../components/sso-connection-form/sso-connection-form.component';
import { SsoEnforcementComponent } from '../../../components/sso-enforcement/sso-enforcement.component';
import { SsoGroupMappingComponent } from '../../../components/sso-group-mapping/sso-group-mapping.component';
import { SsoScimComponent } from '../../../components/sso-scim/sso-scim.component';
import { TeamApi } from 'src/app/team/api/team.api.service';
import { SsoIdpDataComponent } from '../../../components/sso-idp-data/sso-idp-data.component';
import { SsoRulesFormComponent } from '../../../components/sso-rules-form/sso-rules-form.component';
import { SsoHintComponent } from '../../../components/sso-hint/sso-hint.component';
import { SsoStatusComponent } from '../../../components/sso-status/sso-status.component';
import { SsoTestResultComponent } from '../../../components/sso-test-result/sso-test-result.component';
import { TeamLabelComponent } from 'src/app/shared/team-label/team-label.component';
import { SsoState, SsoTestResult } from '../../../model/admin-sso.model';
import { SsoRedirect } from '../../../service/sso-redirect.service';
import { AdminSettingsSsoComponent } from './admin-settings-sso.component';

const mapping = { idSsoGroupMapping: 4, groupValue: 'dev', idTeam: 1, teamName: 'Developers' };

const passedTest: SsoTestResult = {
    isSuccess: true,
    subject: 'sub-1',
    email: 'jana@acme.test',
    emailVerified: true,
    name: 'Jana',
    groups: ['dev', 'ops'],
    hasGroupsClaim: true,
    warnings: []
};

interface SetupOptions {
    state$?: Observable<SsoState>;
    query?: Record<string, string>;
}

describe('AdminSettingsSsoComponent (browser)', () => {
    let api: {
        load$: ReturnType<typeof vi.fn>;
        updateDraft$: ReturnType<typeof vi.fn>;
        startTest$: ReturnType<typeof vi.fn>;
        loadTestResult$: ReturnType<typeof vi.fn>;
        activate$: ReturnType<typeof vi.fn>;
        disable$: ReturnType<typeof vi.fn>;
        updateRules$: ReturnType<typeof vi.fn>;
        loadMappings$: ReturnType<typeof vi.fn>;
        insertMapping$: ReturnType<typeof vi.fn>;
        deleteMapping$: ReturnType<typeof vi.fn>;
        setEnforced$: ReturnType<typeof vi.fn>;
        setScimEnabled$: ReturnType<typeof vi.fn>;
        generateScimToken$: ReturnType<typeof vi.fn>;
        deleteScimToken$: ReturnType<typeof vi.fn>;
        regenerateSpCertificate$: ReturnType<typeof vi.fn>;
    };
    let redirect: { go: ReturnType<typeof vi.fn> };
    let navigate: ReturnType<typeof vi.fn>;

    async function setup({ state$ = of(ssoState()), query = {} }: SetupOptions = {}) {
        api = {
            load$: vi.fn().mockReturnValue(state$),
            updateDraft$: vi.fn().mockReturnValue(of(ssoState())),
            startTest$: vi.fn().mockReturnValue(of({ url: 'https://idp.acme.test/authorize?x=1' })),
            loadTestResult$: vi.fn().mockReturnValue(of(passedTest)),
            activate$: vi.fn().mockReturnValue(of(ssoState({ isEnabled: true }))),
            disable$: vi.fn().mockReturnValue(of(ssoState())),
            updateRules$: vi.fn().mockReturnValue(of(ssoState())),
            loadMappings$: vi.fn().mockReturnValue(of([])),
            insertMapping$: vi.fn().mockReturnValue(of([mapping])),
            deleteMapping$: vi.fn().mockReturnValue(of([])),
            setEnforced$: vi.fn().mockReturnValue(of(ssoState())),
            setScimEnabled$: vi.fn().mockReturnValue(of(ssoState())),
            generateScimToken$: vi.fn().mockReturnValue(of({ token: 'rdscim_new' })),
            deleteScimToken$: vi.fn().mockReturnValue(of(ssoState())),
            regenerateSpCertificate$: vi.fn().mockReturnValue(of(ssoState()))
        };
        redirect = { go: vi.fn() };
        navigate = vi.fn().mockResolvedValue(true);
        await TestBed.configureTestingModule({
            declarations: [
                AdminSettingsSsoComponent,
                SsoIdpDataComponent,
                SsoConnectionFormComponent,
                SsoTestResultComponent,
                SsoStatusComponent,
                SsoHintComponent,
                TeamLabelComponent,
                SsoRulesFormComponent,
                SsoGroupMappingComponent,
                SsoEnforcementComponent,
                SsoScimComponent
            ],
            imports: [
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent
            ],
            providers: [
                { provide: AdminSsoApi, useValue: api },
                { provide: SsoRedirect, useValue: redirect },
                {
                    provide: TeamApi,
                    useValue: {
                        load$: () => of([{ idTeam: 1, name: 'Developers', color: '#fff' }])
                    }
                },
                { provide: Router, useValue: { navigate } },
                {
                    provide: ActivatedRoute,
                    useValue: { snapshot: { queryParamMap: convertToParamMap(query) } }
                },
                provideTablerIcons({
                    IconArrowDownLeft,
                    IconArrowRight,
                    IconArrowUpRight,
                    IconBook,
                    IconChevronRight,
                    IconCircle,
                    IconCircleCheck,
                    IconCopy,
                    IconInfoCircle,
                    IconListCheck,
                    IconLock,
                    IconPlugConnected,
                    IconShield,
                    IconShieldCheck,
                    IconTrash,
                    IconUsersGroup
                })
            ]
        }).compileComponents();
        const fixture = TestBed.createComponent(AdminSettingsSsoComponent);
        fixture.detectChanges();
        return fixture;
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function click(testId: string): void {
        (query(testId)?.querySelector('button') as HTMLButtonElement).click();
    }

    it('shows the data for the identity provider', async () => {
        await setup();

        expect(query('sso-redirect-uri')?.textContent).toContain(
            'https://tracker.acme.test/api/public/sso/oidc/callback'
        );
    });

    it('saves the draft before it starts the test and leaves for the identity provider', async () => {
        await setup();

        click('sso-save-and-test');

        expect(api.updateDraft$).toHaveBeenCalled();
        expect(api.startTest$).toHaveBeenCalled();
        expect(redirect.go).toHaveBeenCalledWith('https://idp.acme.test/authorize?x=1');
    });

    it('leaves no test result in the form before a test ran', async () => {
        await setup();

        expect(document.querySelector('app-sso-test-result')).toBeNull();
    });

    it('shows the test result from the query string and removes the parameter', async () => {
        const fixture = await setup({ query: { test: 'id-1' } });
        fixture.detectChanges();

        expect(api.loadTestResult$).toHaveBeenCalledWith('id-1');
        expect(query('sso-test-success')).not.toBeNull();
        expect(navigate).toHaveBeenCalledWith(
            [],
            expect.objectContaining({ queryParams: { test: null }, replaceUrl: true })
        );
    });

    it('activates a tested draft', async () => {
        const fixture = await setup({ state$: of(ssoState({ isDraftTested: true })) });

        click('sso-activate');
        fixture.detectChanges();

        expect(api.activate$).toHaveBeenCalled();
        expect(query('sso-status')?.textContent).toContain('ADMIN.SSO.STATUS.ACTIVE');
    });

    it('explains a missing license and keeps the turn-off button while SSO is on', async () => {
        await setup({ state$: of(ssoState({ isLicensed: false, isEnabled: true })) });

        expect(query('sso-unlicensed')).not.toBeNull();
        expect(query('sso-disable-unlicensed')).not.toBeNull();
        expect(query('sso-connection')).toBeNull();
    });

    it('keeps the way to stop enforcing without a license', async () => {
        const fixture = await setup({
            state$: of(ssoState({ isLicensed: false, isEnabled: true, isEnforced: true }))
        });
        api.setEnforced$.mockReturnValue(of(ssoState({ isLicensed: false, isEnabled: true })));

        click('sso-enforce-disable');
        fixture.detectChanges();

        expect(api.setEnforced$).toHaveBeenCalledWith({ isEnforced: false });
        expect(query('sso-enforcement')).toBeNull();
    });

    it('shows a generated SCIM token once and reloads the state', async () => {
        const scimOn = { ...ssoState().scim, isEnabled: true };
        const fixture = await setup({ state$: of(ssoState({ scim: scimOn })) });
        api.load$.mockReturnValue(of(ssoState({ scim: { ...scimOn, hasToken: true } })));

        click('sso-scim-generate');
        fixture.detectChanges();

        expect(api.generateScimToken$).toHaveBeenCalled();
        expect(query('sso-scim-token-value')?.textContent).toContain('rdscim_new');
        expect(query('sso-scim-regenerate')).not.toBeNull();
    });

    it('lists what is missing before single sign-on can be turned on', async () => {
        await setup();

        expect(query('sso-checklist')).not.toBeNull();
        expect(query('sso-check-tested')?.getAttribute('data-done')).toBe('false');
        expect(query('sso-check-domains')?.getAttribute('data-done')).toBe('true');
        expect((query('sso-activate')?.querySelector('button') as HTMLButtonElement).disabled).toBe(
            true
        );
    });

    it('applies tested changes of a live connection', async () => {
        await setup({
            state$: of(ssoState({ isEnabled: true, isDraftTested: true, isDraftActive: false }))
        });

        expect(query('sso-checklist')).toBeNull();
        click('sso-apply');

        expect(api.activate$).toHaveBeenCalled();
    });

    it('shows the SAML data for the identity provider once SAML is picked', async () => {
        const fixture = await setup();
        expect(query('sso-redirect-uri')).not.toBeNull();

        const saml = Array.from(
            document.querySelectorAll<HTMLButtonElement>('[data-testid="sso-protocol"] button')
        ).find(button => button.textContent?.includes('SAML'));
        saml?.click();
        fixture.detectChanges();

        expect(query('sso-redirect-uri')).toBeNull();
        expect(query('sso-acs-url')?.textContent).toContain('/api/public/sso/saml/acs');
        expect(query('sso-metadata-url')).not.toBeNull();
    });

    it('offers a new SP certificate only when the key is lost', async () => {
        const fixture = await setup({ state$: of(ssoState({ isSpKeyUndecryptable: true })) });

        click('sso-sp-regenerate');
        fixture.detectChanges();
        await fixture.whenStable();
        const [, accept] = Array.from(
            document.querySelectorAll<HTMLButtonElement>('.ui-confirm-panel button')
        );
        accept.click();

        expect(api.regenerateSpCertificate$).toHaveBeenCalled();
    });

    it('treats a build without SSO routes as unlicensed', async () => {
        await setup({ state$: throwError(() => ({ status: 404 })) });

        expect(query('sso-unlicensed')).not.toBeNull();
        expect(query('sso-disable-unlicensed')).toBeNull();
    });

    it('reports a missing encryption key and blocks editing', async () => {
        await setup({ state$: of(ssoState({ hasEncryptionKey: false })) });

        expect(query('sso-key-missing')).not.toBeNull();
        expect((query('sso-issuer') as HTMLInputElement).disabled).toBe(true);
    });

    it('reports an undecryptable secret', async () => {
        await setup({ state$: of(ssoState({ isSecretUndecryptable: true })) });

        expect(query('sso-secret-undecryptable')).not.toBeNull();
    });

    it('offers the groups from the test result for mapping and saves a mapping', async () => {
        const fixture = await setup({ query: { test: 'id-1' } });
        fixture.detectChanges();

        const suggestions = Array.from(
            document.querySelectorAll('[data-testid="sso-mapping-suggestion"]')
        ).map(element => element.textContent?.trim());
        expect(suggestions).toEqual(['dev', 'ops']);

        const mappingBlock = fixture.debugElement.query(
            debugElement => debugElement.componentInstance instanceof SsoGroupMappingComponent
        ).componentInstance as SsoGroupMappingComponent;
        mappingBlock.add.emit({ groupValue: 'dev', idTeam: 1 });
        fixture.detectChanges();

        expect(api.insertMapping$).toHaveBeenCalledWith({ groupValue: 'dev', idTeam: 1 });
        expect(document.querySelectorAll('[data-testid="sso-mapping-row"]')).toHaveLength(1);
    });
});
