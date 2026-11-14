import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { SSO_DOCS_URL } from '../../constants/sso.constants';
import { SsoState } from '../../model/admin-sso.model';
import { SsoProtocol } from '../../model/sso-protocol.enum';

@Component({
    selector: 'app-sso-status',
    templateUrl: './sso-status.component.html',
    styleUrls: ['./sso-status.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoStatusComponent {
    public readonly state = input.required<SsoState>();
    public readonly hasMappings = input(false);
    public readonly canActivate = input(false);
    public readonly isBusy = input(false);
    public readonly activate = output<void>();
    public readonly disable = output<void>();

    protected readonly docsUrl = SSO_DOCS_URL;
    protected readonly hasDomains = computed(() => this.state().rules.allowedDomains.length > 0);
    protected readonly protocolLabel = computed(() =>
        this.state().active?.protocol === SsoProtocol.Saml ? 'SAML' : 'OIDC'
    );
    protected readonly issuerHost = computed(() => {
        const issuer = this.state().active?.issuer ?? '';
        try {
            return new URL(issuer).host;
        } catch {
            return issuer;
        }
    });
}
