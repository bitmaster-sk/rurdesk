import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from 'src/app/ui/ui.module';
import { SsoTestResult } from '../../model/admin-sso.model';
import { SsoTestAction } from '../../model/sso-test-action.enum';
import { SsoTestResultComponent } from './sso-test-result.component';

const success: SsoTestResult = {
    isSuccess: true,
    subject: 'sub-1',
    email: 'jana@acme.test',
    emailVerified: true,
    name: 'Jana',
    groups: [],
    hasGroupsClaim: false,
    action: SsoTestAction.Create,
    warnings: ['groups_claim_missing']
};

describe('SsoTestResultComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoTestResultComponent],
            imports: [TranslateModule.forRoot(), UiModule]
        }).compileComponents();
    });

    function setup(result: SsoTestResult | null): void {
        const fixture = TestBed.createComponent(SsoTestResultComponent);
        fixture.componentRef.setInput('result', result);
        fixture.detectChanges();
    }

    function query(testId: string): HTMLElement | null {
        return document.querySelector(`[data-testid="${testId}"]`);
    }

    it('shows nothing before a test', () => {
        setup(null);

        expect(query('sso-test-success')).toBeNull();
        expect(query('sso-test-error')).toBeNull();
    });

    it('shows what the identity provider sent, the predicted outcome and the warnings', () => {
        setup(success);

        expect(query('sso-test-success')?.textContent).toContain('jana@acme.test');
        expect(query('sso-test-outcome')?.textContent).toContain('ADMIN.SSO.OUTCOME.CREATE');
        expect(query('sso-test-warning')?.textContent).toContain(
            'ADMIN.SSO.WARNING.GROUPS_CLAIM_MISSING'
        );
    });

    it('shows the reason of a predicted rejection', () => {
        setup({ ...success, action: SsoTestAction.Reject, reason: 'domain_not_allowed' });

        expect(query('sso-test-outcome')?.textContent).toContain(
            'LOGIN.SSO_ERROR.DOMAIN_NOT_ALLOWED'
        );
    });

    it('shows a failed test with its detail', () => {
        setup({ ...success, isSuccess: false, error: 'token_invalid', detail: 'bad signature' });

        expect(query('sso-test-error')?.textContent).toContain(
            'ADMIN.SSO.TEST_ERROR.TOKEN_INVALID'
        );
        expect(query('sso-test-error')?.textContent).toContain('bad signature');
    });
});
