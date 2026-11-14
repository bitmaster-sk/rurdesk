import { AuthMethods } from '../model/auth-methods.model';

export const SSO_START_URL = '/api/public/sso/start';

export const PASSWORD_ONLY: AuthMethods = { password: true, sso: null, ssoUnavailable: false };
