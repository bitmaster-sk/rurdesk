import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { SsoIdpField } from '../../entity/sso-idp-field.entity';
import { SsoState } from '../../model/admin-sso.model';
import { SsoProtocol } from '../../model/sso-protocol.enum';

@Component({
    selector: 'app-sso-idp-data',
    templateUrl: './sso-idp-data.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoIdpDataComponent {
    private readonly clipboard = inject(ClipboardService);

    public readonly state = input.required<SsoState>();

    public readonly protocol = input<SsoProtocol>(SsoProtocol.Oidc);

    protected readonly fields = computed<SsoIdpField[]>(() => {
        const state = this.state();
        if (this.protocol() === SsoProtocol.Saml) {
            return [
                { label: 'ADMIN.SSO.IDP_DATA.ACS_URL', value: state.acsUrl, testId: 'sso-acs-url' },
                {
                    label: 'ADMIN.SSO.IDP_DATA.SP_ENTITY_ID',
                    value: state.spEntityId,
                    testId: 'sso-sp-entity-id'
                },
                {
                    label: 'ADMIN.SSO.IDP_DATA.SP_METADATA_URL',
                    value: state.spEntityId,
                    testId: 'sso-sp-metadata-url'
                }
            ];
        }
        return [
            {
                label: 'ADMIN.SSO.IDP_DATA.REDIRECT_URI',
                value: state.redirectUri,
                testId: 'sso-redirect-uri'
            }
        ];
    });

    protected onCopy(value: string): void {
        void this.clipboard.copy(value);
    }
}
