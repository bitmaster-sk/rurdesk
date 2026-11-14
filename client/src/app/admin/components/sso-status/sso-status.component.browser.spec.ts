import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconBook,
    IconCircle,
    IconCircleCheck,
    IconShield,
    IconShieldCheck,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { UiModule } from 'src/app/ui/ui.module';
import { ssoState } from 'src/testing/sso-state.fixture';
import { SsoState } from '../../model/admin-sso.model';
import { SsoStatusComponent } from './sso-status.component';

describe('SsoStatusComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoStatusComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconComponent],
            providers: [
                provideTablerIcons({
                    IconBook,
                    IconCircle,
                    IconCircleCheck,
                    IconShield,
                    IconShieldCheck
                })
            ]
        }).compileComponents();
    });

    function setup(state: SsoState, canActivate = false, hasMappings = false) {
        const fixture = TestBed.createComponent(SsoStatusComponent);
        fixture.componentRef.setInput('state', state);
        fixture.componentRef.setInput('canActivate', canActivate);
        fixture.componentRef.setInput('hasMappings', hasMappings);
        fixture.detectChanges();
        const outputs = { activate: vi.fn(), disable: vi.fn() };
        fixture.componentInstance.activate.subscribe(outputs.activate);
        fixture.componentInstance.disable.subscribe(outputs.disable);
        return { fixture, ...outputs };
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    function button(testId: string): HTMLButtonElement {
        return query(testId)?.querySelector('button') as HTMLButtonElement;
    }

    it('lists what is done and what is missing while single sign-on is off', () => {
        setup(
            ssoState({ rules: { allowedDomains: [], isAutoCreate: false, adminGroup: '' } }),
            false,
            true
        );

        expect(query('sso-status')?.textContent).toContain('ADMIN.SSO.STATUS.INACTIVE');
        expect(query('sso-check-tested')?.getAttribute('data-done')).toBe('false');
        expect(query('sso-check-domains')?.getAttribute('data-done')).toBe('false');
        expect(query('sso-check-mapped')?.getAttribute('data-done')).toBe('true');
        expect(button('sso-activate').disabled).toBe(true);
    });

    it('turns on a tested connection', () => {
        const { activate } = setup(ssoState({ isDraftTested: true }), true);

        expect(query('sso-check-tested')?.getAttribute('data-done')).toBe('true');
        button('sso-activate').click();

        expect(activate).toHaveBeenCalled();
    });

    it('describes a live connection and turns it off after the confirmation', async () => {
        const { fixture, disable } = setup(
            ssoState({
                isEnabled: true,
                isEnforced: true,
                active: { ...ssoState().draft!, issuer: 'https://idp.acme.test/realms/acme' }
            })
        );

        expect(query('sso-status')?.textContent).toContain('ADMIN.SSO.STATUS.ACTIVE');
        expect(query('sso-status-detail')?.textContent).toContain('OIDC · idp.acme.test');
        expect(query('sso-status-detail')?.textContent).toContain('ADMIN.SSO.STATUS.ENFORCED');
        expect(query('sso-checklist')).toBeNull();

        button('sso-disable').click();
        fixture.detectChanges();
        await fixture.whenStable();
        const [, accept] = Array.from(
            document.querySelectorAll<HTMLButtonElement>('.ui-confirm-panel button')
        );
        accept.click();

        expect(disable).toHaveBeenCalled();
    });
});
