import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { IconInfoCircle, TablerIconComponent, provideTablerIcons } from '@tabler/icons-angular';
import { UiModule } from 'src/app/ui/ui.module';
import { SsoHintComponent } from './sso-hint.component';

describe('SsoHintComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoHintComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconComponent],
            providers: [provideTablerIcons({ IconInfoCircle })]
        }).compileComponents();
    });

    it('explains itself to keyboard and screen reader users', async () => {
        const fixture = TestBed.createComponent(SsoHintComponent);
        fixture.componentRef.setInput('text', 'ADMIN.SSO.RULES.DOMAINS_INFO');
        fixture.detectChanges();
        const hint = document.querySelector('[data-testid="sso-hint"]') as HTMLElement;

        expect(hint.getAttribute('aria-label')).toBe('ADMIN.SSO.RULES.DOMAINS_INFO');
        expect(hint.tabIndex).toBe(0);

        hint.dispatchEvent(new Event('focus'));
        await vi.waitFor(() =>
            expect(document.body.textContent).toContain('ADMIN.SSO.RULES.DOMAINS_INFO')
        );
    });
});
