import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { IconInfoCircle, TablerIconComponent, provideTablerIcons } from '@tabler/icons-angular';
import { UiModule } from 'src/app/ui/ui.module';
import { ssoState } from 'src/testing/sso-state.fixture';
import { SsoState } from '../../model/admin-sso.model';
import { SsoHintComponent } from '../sso-hint/sso-hint.component';
import { SsoEnforcementComponent } from './sso-enforcement.component';

const breakGlass = { idUser: 7, name: 'Vault', email: 'vault@acme.test', lastLoginAt: null };

describe('SsoEnforcementComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoEnforcementComponent, SsoHintComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconComponent],
            providers: [provideTablerIcons({ IconInfoCircle })]
        }).compileComponents();
    });

    function setup(state: SsoState) {
        const fixture = TestBed.createComponent(SsoEnforcementComponent);
        fixture.componentRef.setInput('state', state);
        fixture.detectChanges();
        const enforcedChange = vi.fn();
        fixture.componentInstance.enforcedChange.subscribe(enforcedChange);
        return { fixture, enforcedChange };
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function button(testId: string): HTMLButtonElement {
        return query(testId)?.querySelector('button') as HTMLButtonElement;
    }

    it('blocks enforcement without a break-glass account', () => {
        setup(ssoState({ isEnabled: true }));

        expect(button('sso-enforce-enable').disabled).toBe(true);
        expect(query('sso-enforce-needs-break-glass')?.textContent).toContain(
            'ADMIN.SSO.ENFORCE.NO_ACCOUNTS'
        );
        expect(query('sso-break-glass-table')).toBeNull();
    });

    it('blocks enforcement while single sign-on is off', () => {
        setup(ssoState({ breakGlassAccounts: [breakGlass] }));

        expect(button('sso-enforce-enable').disabled).toBe(true);
        expect(query('sso-enforce-needs-active')).not.toBeNull();
        expect(query('sso-break-glass-table')).toBeNull();
    });

    it('lists the break-glass accounts and enforces after the confirmation', async () => {
        const { fixture, enforcedChange } = setup(
            ssoState({ isEnabled: true, breakGlassAccounts: [breakGlass] })
        );

        expect(document.querySelectorAll('[data-testid="sso-break-glass-row"]')).toHaveLength(1);
        expect(query('sso-break-glass-row')?.textContent).toContain('ADMIN.SSO.ENFORCE.NEVER');

        button('sso-enforce-enable').click();
        fixture.detectChanges();
        await fixture.whenStable();
        expect(enforcedChange).not.toHaveBeenCalled();
        const [, accept] = Array.from(
            document.querySelectorAll<HTMLButtonElement>('.ui-confirm-panel button')
        );
        accept.click();

        expect(enforcedChange).toHaveBeenCalledWith(true);
    });

    it('stops enforcing without a license', () => {
        const { enforcedChange } = setup(
            ssoState({ isLicensed: false, isEnabled: true, isEnforced: true })
        );

        button('sso-enforce-disable').click();

        expect(enforcedChange).toHaveBeenCalledWith(false);
    });
});
