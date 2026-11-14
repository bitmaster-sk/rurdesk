import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { SILENCE_ERROR_TOAST } from 'src/app/core/request-context';
import { SsoProtocol } from '../model/sso-protocol.enum';
import { AdminSsoApi } from './admin-sso.api.service';

function build() {
    const http = {
        get: vi.fn().mockReturnValue(of({})),
        post: vi.fn().mockReturnValue(of({})),
        put: vi.fn().mockReturnValue(of({})),
        delete: vi.fn().mockReturnValue(of([]))
    };
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: http }] });
    return { api: runInInjectionContext(injector, () => new AdminSsoApi()), http };
}

describe('AdminSsoApi', () => {
    it('loads the state without an error toast, because the free build has no SSO routes', () => {
        const { api, http } = build();

        api.load$().subscribe();

        const [url, options] = http.get.mock.calls[0];
        expect(url).toBe('/api/private/admin/sso');
        expect(options.context.get(SILENCE_ERROR_TOAST)).toBe(true);
    });

    it('calls the admin SSO endpoints', () => {
        const { api, http } = build();
        const draft = {
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
        };

        api.updateDraft$(draft).subscribe();
        api.startTest$().subscribe();
        api.loadTestResult$('a/b').subscribe();
        api.activate$().subscribe();
        api.disable$().subscribe();
        api.setEnforced$({ isEnforced: true }).subscribe();
        api.setScimEnabled$({ isEnabled: true }).subscribe();
        api.generateScimToken$().subscribe();
        api.deleteScimToken$().subscribe();
        api.regenerateSpCertificate$().subscribe();
        api.updateRules$({
            allowedDomains: ['acme.test'],
            isAutoCreate: true,
            adminGroup: ''
        }).subscribe();

        expect(http.put).toHaveBeenCalledWith('/api/private/admin/sso/draft', draft);
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/test', {});
        expect(http.get).toHaveBeenCalledWith('/api/private/admin/sso/test/a%2Fb');
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/activate', {});
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/disable', {});
        expect(http.put).toHaveBeenCalledWith('/api/private/admin/sso/enforce', {
            isEnforced: true
        });
        expect(http.put).toHaveBeenCalledWith('/api/private/admin/sso/scim', { isEnabled: true });
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/scim/token', {});
        expect(http.delete).toHaveBeenCalledWith('/api/private/admin/sso/scim/token');
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/sp-certificate', {});
        expect(http.put).toHaveBeenCalledWith('/api/private/admin/sso/settings', {
            allowedDomains: ['acme.test'],
            isAutoCreate: true,
            adminGroup: ''
        });
    });

    it('calls the mapping and identity endpoints', () => {
        const { api, http } = build();

        api.loadMappings$().subscribe();
        api.insertMapping$({ groupValue: 'dev', idTeam: 3 }).subscribe();
        api.deleteMapping$(4).subscribe();
        api.loadIdentities$(5).subscribe();
        api.unlinkIdentity$(5).subscribe();
        api.moveIdentity$(5, { idTargetUser: 6 }).subscribe();

        expect(http.get.mock.calls[0][0]).toBe('/api/private/admin/sso/mapping');
        expect(http.get.mock.calls[0][1].context.get(SILENCE_ERROR_TOAST)).toBe(true);
        expect(http.post).toHaveBeenCalledWith('/api/private/admin/sso/mapping', {
            groupValue: 'dev',
            idTeam: 3
        });
        expect(http.delete.mock.calls[0][0]).toBe('/api/private/admin/sso/mapping/4');
        expect(http.get.mock.calls[1][0]).toBe('/api/private/admin/sso/identity/5');
        expect(http.delete.mock.calls[1][0]).toBe('/api/private/admin/sso/identity/5');
        expect(http.post.mock.calls[1][0]).toBe('/api/private/admin/sso/identity/5/move');
        expect(http.post.mock.calls[1][1]).toEqual({ idTargetUser: 6 });
    });
});
