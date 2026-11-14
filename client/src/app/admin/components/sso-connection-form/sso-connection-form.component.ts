import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    computed,
    effect,
    inject,
    input,
    output,
    untracked
} from '@angular/core';
import { outputFromObservable, takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { AbstractControl, FormBuilder, ValidationErrors, Validators } from '@angular/forms';
import {
    SSO_DEFAULT_EMAIL_CLAIM,
    SSO_DEFAULT_GROUPS_CLAIM,
    SSO_DEFAULT_NAME_ATTRIBUTE,
    SSO_PROTOCOL_OPTIONS
} from '../../constants/sso.constants';
import { SaveSsoDraftReq, SsoConnection } from '../../model/admin-sso.model';
import { SsoProtocol } from '../../model/sso-protocol.enum';

@Component({
    selector: 'app-sso-connection-form',
    templateUrl: './sso-connection-form.component.html',
    styleUrls: ['./sso-connection-form.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoConnectionFormComponent {
    private readonly fb = inject(FormBuilder);
    private readonly changeDetector = inject(ChangeDetectorRef);

    public readonly connection = input<SsoConnection | null>(null);
    public readonly isDisabled = input(false);
    public readonly canTest = input(false);
    public readonly isLive = input(false);
    public readonly isDraftActive = input(false);
    public readonly canApply = input(false);
    public readonly save = output<SaveSsoDraftReq>();
    public readonly saveAndTest = output<SaveSsoDraftReq>();
    public readonly apply = output<void>();

    protected readonly protocolOptions = SSO_PROTOCOL_OPTIONS;

    private readonly storedConnection = computed(() => this.connection(), {
        equal: (a, b) => JSON.stringify(a) === JSON.stringify(b)
    });

    protected readonly form = this.fb.nonNullable.group(
        {
            protocol: [SsoProtocol.Oidc],
            issuer: ['', [Validators.maxLength(500)]],
            clientId: ['', [Validators.maxLength(500)]],
            clientSecret: ['', [Validators.maxLength(2000)]],
            hasGroupsScope: [false],
            emailClaim: [SSO_DEFAULT_EMAIL_CLAIM, [Validators.maxLength(300)]],
            groupsClaim: [SSO_DEFAULT_GROUPS_CLAIM, [Validators.maxLength(300)]],
            metadataUrl: ['', [Validators.maxLength(2000)]],
            metadataXml: ['', [Validators.maxLength(500000)]],
            nameAttribute: [SSO_DEFAULT_NAME_ATTRIBUTE, [Validators.maxLength(300)]],
            subjectAttribute: ['', [Validators.maxLength(300)]]
        },
        { validators: control => this.validateProtocol(control) }
    );

    public readonly protocolChange = outputFromObservable(this.form.controls.protocol.valueChanges);

    protected readonly protocol = toSignal(this.form.controls.protocol.valueChanges, {
        initialValue: SsoProtocol.Oidc
    });
    protected readonly SsoProtocol = SsoProtocol;
    protected readonly isPending = computed(() => this.isLive() && !this.isDraftActive());

    public constructor() {
        effect(() => {
            const connection = this.storedConnection();
            untracked(() => this.fill(connection));
        });
        effect(() => {
            if (this.isDisabled()) {
                this.form.disable({ emitEvent: false });
            } else {
                this.form.enable({ emitEvent: false });
            }
        });
        this.form.controls.metadataXml.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(metadataXml => {
                if (metadataXml.trim() && this.form.controls.metadataUrl.value) {
                    this.form.controls.metadataUrl.setValue('');
                }
            });
    }

    protected onSave(): void {
        if (this.form.valid) {
            this.save.emit(this.toRequest());
        }
    }

    protected onSaveAndTest(): void {
        if (this.form.valid && this.canTest()) {
            this.saveAndTest.emit(this.toRequest());
        }
    }

    protected async onMetadataFile(event: Event): Promise<void> {
        const file = (event.target as HTMLInputElement).files?.[0];
        if (file) {
            this.form.patchValue({ metadataXml: await file.text(), metadataUrl: '' });
            this.changeDetector.markForCheck();
        }
    }

    private validateProtocol(control: AbstractControl): ValidationErrors | null {
        const value = control.getRawValue() as SaveSsoDraftReq;
        if (value.protocol === SsoProtocol.Saml) {
            const keepsUploaded = this.hasUploadedMetadata() && !value.metadataUrl.trim();
            return value.metadataUrl.trim() || value.metadataXml.trim() || keepsUploaded
                ? null
                : { metadataRequired: true };
        }
        const hasSecret = !!value.clientSecret || !!this.connection()?.hasClientSecret;
        return value.issuer.trim() && value.clientId.trim() && hasSecret
            ? null
            : { oidcRequired: true };
    }

    private hasUploadedMetadata(): boolean {
        const connection = this.connection();
        return (
            connection?.protocol === SsoProtocol.Saml &&
            connection.hasMetadata &&
            !connection.metadataUrl
        );
    }

    private fill(connection: SsoConnection | null): void {
        if (connection) {
            this.form.patchValue({
                protocol: connection.protocol,
                issuer: connection.protocol === SsoProtocol.Oidc ? connection.issuer : '',
                clientId: connection.clientId,
                clientSecret: '',
                hasGroupsScope: connection.hasGroupsScope,
                emailClaim: connection.emailClaim,
                groupsClaim: connection.groupsClaim,
                metadataUrl: connection.metadataUrl,
                metadataXml: '',
                nameAttribute: connection.nameAttribute || SSO_DEFAULT_NAME_ATTRIBUTE,
                subjectAttribute: connection.subjectAttribute
            });
        }
        this.form.updateValueAndValidity();
    }

    private toRequest(): SaveSsoDraftReq {
        return this.form.getRawValue();
    }
}
