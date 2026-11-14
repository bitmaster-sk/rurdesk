import { SsoProtocol } from './sso-protocol.enum';
import { SsoTestAction } from './sso-test-action.enum';

export interface SsoConnection {
    protocol: SsoProtocol;
    issuer: string;
    clientId: string;
    hasClientSecret: boolean;
    hasGroupsScope: boolean;
    emailClaim: string;
    groupsClaim: string;
    metadataUrl: string;
    hasMetadata: boolean;
    metadataFetchedAt: string | null;
    nameAttribute: string;
    subjectAttribute: string;
    updateAt: string;
}

export interface SsoRules {
    allowedDomains: string[];
    isAutoCreate: boolean;
    adminGroup: string;
}

export interface SsoState {
    isLicensed: boolean;
    hasEncryptionKey: boolean;
    isSecretUndecryptable: boolean;
    isSpKeyUndecryptable: boolean;
    publicBaseUrl: string;
    isPublicBaseUrlValid: boolean;
    redirectUri: string;
    acsUrl: string;
    spEntityId: string;
    draft: SsoConnection | null;
    active: SsoConnection | null;
    isDraftTested: boolean;
    isDraftActive: boolean;
    isEnabled: boolean;
    rules: SsoRules;
    isEnforced: boolean;
    breakGlassAccounts: SsoBreakGlassAccount[];
    scim: SsoScimState;
}

export interface SsoScimState {
    isEnabled: boolean;
    hasToken: boolean;
    tokenCreateAt: string | null;
    lastUsedAt: string | null;
    baseUrl: string;
}

export interface SetSsoScimEnabledReq {
    isEnabled: boolean;
}

export interface SsoScimTokenRes {
    token: string;
}

export interface SsoBreakGlassAccount {
    idUser: number;
    name: string;
    email: string;
    lastLoginAt: string | null;
}

export interface SetSsoEnforcedReq {
    isEnforced: boolean;
}

export interface SaveSsoDraftReq {
    protocol: SsoProtocol;
    issuer: string;
    clientId: string;
    clientSecret: string;
    hasGroupsScope: boolean;
    emailClaim: string;
    groupsClaim: string;
    metadataUrl: string;
    metadataXml: string;
    nameAttribute: string;
    subjectAttribute: string;
}

export interface UpdateSsoRulesReq {
    allowedDomains: string[];
    isAutoCreate: boolean;
    adminGroup: string;
}

export interface StartSsoTestRes {
    url: string;
}

export interface SsoTestResult {
    isSuccess: boolean;
    error?: string;
    detail?: string;
    subject: string;
    email: string;
    emailVerified: boolean | null;
    name: string;
    groups: string[];
    hasGroupsClaim: boolean;
    action?: SsoTestAction;
    reason?: string;
    warnings: string[];
}

export interface SsoGroupMapping {
    idSsoGroupMapping: number;
    groupValue: string;
    idTeam: number;
    teamName: string;
}

export interface InsertSsoMappingReq {
    groupValue: string;
    idTeam: number;
}

export interface SsoIdentity {
    idUserIdentity: number;
    issuer: string;
    subject: string;
    createAt: string;
}

export interface MoveSsoIdentityReq {
    idTargetUser: number;
}
