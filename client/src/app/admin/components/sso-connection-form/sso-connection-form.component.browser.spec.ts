import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconArrowDownLeft,
    IconArrowUpRight,
    IconChevronRight,
    IconInfoCircle,
    IconPlugConnected,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { UiModule } from 'src/app/ui/ui.module';
import { ssoState } from 'src/testing/sso-state.fixture';
import { SsoConnection } from '../../model/admin-sso.model';
import { SsoProtocol } from '../../model/sso-protocol.enum';
import { SsoHintComponent } from '../sso-hint/sso-hint.component';
import { SsoConnectionFormComponent } from './sso-connection-form.component';

describe('SsoConnectionFormComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoConnectionFormComponent, SsoHintComponent],
            imports: [
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent
            ],
            providers: [
                provideTablerIcons({
                    IconPlugConnected,
                    IconArrowUpRight,
                    IconArrowDownLeft,
                    IconInfoCircle,
                    IconChevronRight
                })
            ]
        }).compileComponents();
    });

    function setup(connection: SsoConnection | null = ssoState().draft, canTest = true) {
        const fixture = TestBed.createComponent(SsoConnectionFormComponent);
        fixture.componentRef.setInput('connection', connection);
        fixture.componentRef.setInput('canTest', canTest);
        fixture.detectChanges();
        const save = vi.fn();
        const saveAndTest = vi.fn();
        fixture.componentInstance.save.subscribe(save);
        fixture.componentInstance.saveAndTest.subscribe(saveAndTest);
        return { fixture, save, saveAndTest };
    }

    function input(testId: string): HTMLInputElement {
        return document.querySelector(`[data-testid="${testId}"]`) as HTMLInputElement;
    }

    function type(testId: string, value: string): void {
        input(testId).value = value;
        input(testId).dispatchEvent(new Event('input'));
    }

    function click(testId: string): void {
        (document.querySelector(`[data-testid="${testId}"] button`) as HTMLButtonElement).click();
    }

    it('fills the saved draft and keeps the secret empty', () => {
        setup();

        expect(input('sso-issuer').value).toBe('https://idp.acme.test');
        expect(input('sso-client-id').value).toBe('rurdesk');
        expect(input('sso-client-secret').value).toBe('');
    });

    it('sends an empty secret to keep the saved one', () => {
        const { save } = setup();

        click('sso-save-draft');

        expect(save).toHaveBeenCalledWith({
            protocol: SsoProtocol.Oidc,
            issuer: 'https://idp.acme.test',
            clientId: 'rurdesk',
            clientSecret: '',
            hasGroupsScope: false,
            emailClaim: 'email',
            groupsClaim: 'groups',
            metadataUrl: '',
            metadataXml: '',
            nameAttribute: 'name',
            subjectAttribute: ''
        });
    });

    it('needs a secret for the first draft', () => {
        const { fixture, save } = setup(null);
        type('sso-issuer', 'https://idp.acme.test');
        type('sso-client-id', 'rurdesk');
        fixture.detectChanges();

        click('sso-save-draft');
        expect(save).not.toHaveBeenCalled();

        type('sso-client-secret', 's3cret');
        fixture.detectChanges();
        click('sso-save-draft');
        expect(save).toHaveBeenCalled();
    });

    function pickSaml(fixture: ReturnType<typeof setup>['fixture']): void {
        Array.from(
            document.querySelectorAll<HTMLButtonElement>('[data-testid="sso-protocol"] button')
        )
            .find(button => button.textContent?.includes('SAML'))
            ?.click();
        fixture.detectChanges();
    }

    it('switches to the SAML fields and needs metadata', () => {
        const { fixture, save } = setup(null);
        pickSaml(fixture);

        expect(input('sso-issuer')).toBeNull();
        click('sso-save-draft');
        expect(save).not.toHaveBeenCalled();

        type('sso-metadata-url', 'https://idp.acme.test/metadata');
        type('sso-subject-attribute', 'objectId');
        fixture.detectChanges();
        click('sso-save-draft');

        expect(save).toHaveBeenCalledWith(
            expect.objectContaining({
                protocol: SsoProtocol.Saml,
                metadataUrl: 'https://idp.acme.test/metadata',
                subjectAttribute: 'objectId',
                nameAttribute: 'name'
            })
        );
    });

    it('keeps uploaded SAML metadata without sending it again', () => {
        const { save } = setup({
            ...ssoState().draft!,
            protocol: SsoProtocol.Saml,
            issuer: 'https://idp.acme.test/entity',
            hasMetadata: true
        });

        expect(document.querySelector('[data-testid="sso-idp-entity-id"]')?.textContent).toContain(
            'https://idp.acme.test/entity'
        );
        click('sso-save-draft');

        expect(save).toHaveBeenCalledWith(
            expect.objectContaining({
                protocol: SsoProtocol.Saml,
                metadataXml: '',
                metadataUrl: ''
            })
        );
    });

    it('clears the metadata URL when metadata XML is pasted', () => {
        const { fixture, save } = setup({
            ...ssoState().draft!,
            protocol: SsoProtocol.Saml,
            metadataUrl: 'https://idp.acme.test/metadata',
            hasMetadata: true
        });
        expect(input('sso-metadata-url').value).toBe('https://idp.acme.test/metadata');

        type('sso-metadata-xml', '<EntityDescriptor/>');
        fixture.detectChanges();

        expect(input('sso-metadata-url').value).toBe('');
        click('sso-save-draft');
        expect(save).toHaveBeenCalledWith(
            expect.objectContaining({ metadataXml: '<EntityDescriptor/>', metadataUrl: '' })
        );
    });

    function isEnabled(testId: string): boolean {
        return !(document.querySelector(`[data-testid="${testId}"] button`) as HTMLButtonElement)
            .disabled;
    }

    it('enables saving once a metadata file is uploaded', async () => {
        const { fixture } = setup(null);
        pickSaml(fixture);
        expect(isEnabled('sso-save-draft')).toBe(false);

        const file = new File(['<EntityDescriptor/>'], 'idp.xml', { type: 'application/xml' });
        file.text = () =>
            new Promise(resolve => setTimeout(() => resolve('<EntityDescriptor/>'), 50));
        Object.defineProperty(input('sso-metadata-file'), 'files', { value: [file] });
        input('sso-metadata-file').dispatchEvent(new Event('change'));

        await vi.waitFor(() => expect(isEnabled('sso-save-draft')).toBe(true));
        expect(isEnabled('sso-save-and-test')).toBe(true);
        expect((input('sso-metadata-xml') as unknown as HTMLTextAreaElement).value).toBe(
            '<EntityDescriptor/>'
        );
    });

    it('keeps unsaved edits when the same draft arrives again', () => {
        const { fixture } = setup();
        type('sso-issuer', 'https://other.acme.test');
        type('sso-client-secret', 'new-secret');

        fixture.componentRef.setInput('connection', { ...ssoState().draft! });
        fixture.detectChanges();

        expect(input('sso-issuer').value).toBe('https://other.acme.test');
        expect(input('sso-client-secret').value).toBe('new-secret');
    });

    it('fills a changed draft', () => {
        const { fixture } = setup();
        type('sso-issuer', 'https://other.acme.test');

        fixture.componentRef.setInput('connection', {
            ...ssoState().draft!,
            issuer: 'https://saved.acme.test'
        });
        fixture.detectChanges();

        expect(input('sso-issuer').value).toBe('https://saved.acme.test');
    });

    it('offers save and test only when testing is possible', () => {
        const { saveAndTest } = setup(ssoState().draft, false);

        click('sso-save-and-test');

        expect(saveAndTest).not.toHaveBeenCalled();
    });

    function setLive(
        fixture: ReturnType<typeof setup>['fixture'],
        isDraftActive: boolean,
        canApply: boolean
    ) {
        fixture.componentRef.setInput('isLive', true);
        fixture.componentRef.setInput('isDraftActive', isDraftActive);
        fixture.componentRef.setInput('canApply', canApply);
        fixture.detectChanges();
    }

    function text(testId: string): string {
        return document.querySelector(`[data-testid="${testId}"]`)?.textContent ?? '';
    }

    it('applies tested changes of a live connection', () => {
        const { fixture } = setup();
        const apply = vi.fn();
        fixture.componentInstance.apply.subscribe(apply);
        setLive(fixture, false, true);

        expect(text('sso-pending')).toContain('ADMIN.SSO.CONNECTION.PENDING_TESTED');
        click('sso-apply');

        expect(apply).toHaveBeenCalled();
    });

    it('asks for a test before changes of a live connection can be applied', () => {
        const { fixture } = setup();
        setLive(fixture, false, false);

        expect(text('sso-pending')).toContain('ADMIN.SSO.CONNECTION.PENDING_UNTESTED');
        expect(document.querySelector('[data-testid="sso-apply"]')).toBeNull();
    });

    it('offers nothing to apply while the live connection matches the draft', () => {
        const { fixture } = setup();
        setLive(fixture, true, true);

        expect(text('sso-pending').trim()).toBe('');
        expect(document.querySelector('[data-testid="sso-apply"]')).toBeNull();
    });
});
