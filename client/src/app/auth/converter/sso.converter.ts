import { SSO_START_URL } from '../constants/sso.constants';
import { SsoErrorCode } from '../model/sso-error-code.enum';

export abstract class SsoConverter {
    private static readonly errorCodes: ReadonlySet<string> = new Set(Object.values(SsoErrorCode));

    public static toErrorKey(code: string): string {
        const known = SsoConverter.errorCodes.has(code) ? code : 'unknown';
        return `LOGIN.SSO_ERROR.${known.toUpperCase()}`;
    }

    public static toCode(fragment: string | null): string | null {
        return new URLSearchParams(fragment ?? '').get('code');
    }

    public static toStartUrl(isExtended: boolean): string {
        return isExtended ? `${SSO_START_URL}?extended=true` : SSO_START_URL;
    }
}
