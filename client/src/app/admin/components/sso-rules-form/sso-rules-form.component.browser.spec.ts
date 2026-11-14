import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconInfoCircle,
    IconListCheck,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { UiModule } from 'src/app/ui/ui.module';
import { SsoHintComponent } from '../sso-hint/sso-hint.component';
import { SsoRulesFormComponent } from './sso-rules-form.component';

describe('SsoRulesFormComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoRulesFormComponent, SsoHintComponent],
            imports: [
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent
            ],
            providers: [provideTablerIcons({ IconListCheck, IconInfoCircle })]
        }).compileComponents();
    });

    it('shows one domain per line and saves the parsed list', () => {
        const fixture = TestBed.createComponent(SsoRulesFormComponent);
        fixture.componentRef.setInput('rules', {
            allowedDomains: ['acme.test', 'b.acme.test'],
            isAutoCreate: false,
            adminGroup: 'admins'
        });
        fixture.detectChanges();
        const save = vi.fn();
        fixture.componentInstance.save.subscribe(save);
        const domains = document.querySelector(
            '[data-testid="sso-domains"]'
        ) as HTMLTextAreaElement;

        expect(domains.value).toBe('acme.test\nb.acme.test');
        expect(domains.classList).toContain('ui-input');

        domains.value = 'acme.test, c.test';
        domains.dispatchEvent(new Event('input'));
        (document.querySelector('[data-testid="sso-auto-create"]') as HTMLInputElement).click();
        const adminGroup = document.querySelector(
            '[data-testid="sso-admin-group"]'
        ) as HTMLInputElement;
        expect(adminGroup.value).toBe('admins');
        adminGroup.value = ' rurdesk-admins ';
        adminGroup.dispatchEvent(new Event('input'));
        fixture.detectChanges();
        (
            document.querySelector('[data-testid="sso-save-rules"] button') as HTMLButtonElement
        ).click();

        expect(save).toHaveBeenCalledWith({
            allowedDomains: ['acme.test', 'c.test'],
            isAutoCreate: true,
            adminGroup: 'rurdesk-admins'
        });
    });

    it('keeps unsaved edits when the same rules arrive again', () => {
        const rules = { allowedDomains: ['acme.test'], isAutoCreate: false, adminGroup: '' };
        const fixture = TestBed.createComponent(SsoRulesFormComponent);
        fixture.componentRef.setInput('rules', rules);
        fixture.detectChanges();
        const domains = document.querySelector(
            '[data-testid="sso-domains"]'
        ) as HTMLTextAreaElement;
        domains.value = 'acme.test\nnew.test';
        domains.dispatchEvent(new Event('input'));

        fixture.componentRef.setInput('rules', { ...rules, allowedDomains: ['acme.test'] });
        fixture.detectChanges();
        expect(domains.value).toBe('acme.test\nnew.test');

        fixture.componentRef.setInput('rules', { ...rules, allowedDomains: ['saved.test'] });
        fixture.detectChanges();
        expect(domains.value).toBe('saved.test');
    });
});
