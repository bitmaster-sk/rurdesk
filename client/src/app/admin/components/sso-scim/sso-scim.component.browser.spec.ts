import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconArrowUpRight,
    IconCopy,
    IconInfoCircle,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { UiModule } from 'src/app/ui/ui.module';
import { ssoState } from 'src/testing/sso-state.fixture';
import { SsoState } from '../../model/admin-sso.model';
import { SsoHintComponent } from '../sso-hint/sso-hint.component';
import { SsoScimComponent } from './sso-scim.component';

const withToken = {
    isEnabled: true,
    hasToken: true,
    tokenCreateAt: '2026-11-10T10:00:00Z',
    lastUsedAt: null,
    baseUrl: 'https://tracker.acme.test/api/scim/v2'
};

describe('SsoScimComponent (browser)', () => {
    let copy: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        copy = vi.fn().mockResolvedValue(true);
        await TestBed.configureTestingModule({
            declarations: [SsoScimComponent, SsoHintComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconComponent],
            providers: [
                { provide: ClipboardService, useValue: { copy } },
                provideTablerIcons({ IconCopy, IconArrowUpRight, IconInfoCircle })
            ]
        }).compileComponents();
    });

    function setup(state: SsoState, revealedToken: string | null = null) {
        const fixture = TestBed.createComponent(SsoScimComponent);
        fixture.componentRef.setInput('state', state);
        fixture.componentRef.setInput('revealedToken', revealedToken);
        fixture.detectChanges();
        const outputs = { enabledChange: vi.fn(), generateToken: vi.fn() };
        fixture.componentInstance.enabledChange.subscribe(outputs.enabledChange);
        fixture.componentInstance.generateToken.subscribe(outputs.generateToken);
        return outputs;
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function button(testId: string): HTMLButtonElement {
        return query(testId)?.querySelector('button') as HTMLButtonElement;
    }

    function toggle(): HTMLInputElement {
        return query('sso-scim-toggle') as HTMLInputElement;
    }

    it('turns SCIM on and shows no token data while it is off', () => {
        const { enabledChange } = setup(ssoState());

        expect(query('sso-scim-base-url')).toBeNull();
        toggle().click();

        expect(enabledChange).toHaveBeenCalledWith(true);
    });

    it('offers a token once SCIM is on', () => {
        const { generateToken } = setup(
            ssoState({ scim: { ...withToken, hasToken: false, tokenCreateAt: null } })
        );

        expect(query('sso-scim-base-url')?.textContent).toContain('/api/scim/v2');
        expect(query('sso-scim-last-used')?.textContent).toContain('ADMIN.SSO.SCIM.NO_TOKEN');
        button('sso-scim-generate').click();

        expect(generateToken).toHaveBeenCalled();
    });

    it('shows a new token once and copies it', () => {
        setup(ssoState({ scim: withToken }), 'rdscim_secret');

        expect(query('sso-scim-token-value')?.textContent).toContain('rdscim_secret');
        expect(query('sso-scim-last-used')?.textContent).toContain('ADMIN.SSO.SCIM.TOKEN_INFO');

        (query('sso-scim-token')?.querySelector('ui-button button') as HTMLButtonElement).click();
        expect(copy).toHaveBeenCalledWith('rdscim_secret');
    });

    it('keeps only the turn-off actions without a license', () => {
        const { enabledChange } = setup(ssoState({ isLicensed: false, scim: withToken }));

        expect(button('sso-scim-regenerate').disabled).toBe(true);
        expect(button('sso-scim-delete-token').disabled).toBe(false);
        expect(toggle().disabled).toBe(false);
        toggle().click();

        expect(enabledChange).toHaveBeenCalledWith(false);
    });

    it('cannot turn SCIM on without a license', () => {
        setup(ssoState({ isLicensed: false }));

        expect(toggle().disabled).toBe(true);
    });
});
