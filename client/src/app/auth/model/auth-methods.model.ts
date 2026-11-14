export interface SsoMethod {
    protocol: string;
    isEnforced: boolean;
    canStaySignedIn: boolean;
}

export interface AuthMethods {
    password: boolean;
    sso: SsoMethod | null;
    ssoUnavailable: boolean;
}
