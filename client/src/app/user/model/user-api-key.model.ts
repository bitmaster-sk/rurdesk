export interface UserApiKey {
    idApiKey: number;
    idUser: number;
    name: string;
    createdAt: string;
    expiresAt: string | null;
    lastUsedAt: string | null;
}

export interface CreatedUserApiKey extends UserApiKey {
    rawKey: string;
}

export interface CreateUserApiKeyReq {
    name: string;
    expiresAt: string | null;
}
