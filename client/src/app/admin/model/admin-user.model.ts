export interface AdminUser {
    idUser: number;
    name: string;
    email: string;
    colorAvatarBg: string;
    isBot: boolean;
    isAdmin: boolean;
}

export interface AdminCreateUserReq {
    name: string;
    email?: string;
    password?: string;
    isBot: boolean;
    isAdmin?: boolean;
    colorAvatarBg?: string;
}

export interface AdminCreateUserRes extends AdminUser {
    rawKey?: string;
}

// Admin edit. Every field optional — only the present ones are applied. For a
// bot only `name` is sent (no email/admin). Reuses PATCH /admin/user/:idUser.
export interface AdminUpdateUserReq {
    name?: string;
    email?: string;
    isAdmin?: boolean;
    colorAvatarBg?: string;
}

// Emitted by the create-user dialog. For a bot we may also have created its
// gateway in the same step, so the freshly-minted tracker→gateway token rides
// along to be revealed once in the keys window.
export interface UserCreatedEvent {
    user: AdminCreateUserRes;
    gatewayToken: string | null;
}

export interface BotApiKey {
    idApiKey: number;
    idUser: number;
    name: string;
    rateLimitOverride: number | null;
    createdAt: string;
    expiresAt: string | null;
    lastUsedAt: string | null;
}

export interface CreateBotKeyRes extends BotApiKey {
    rawKey: string;
}

export interface BotGateway {
    idBotGateway: number;
    idUserBot: number;
    gatewayUrl: string;
    configJson: string;
    createdAt: string;
}

export interface CreateBotGatewayRes extends BotGateway {
    trackerToGatewayToken: string;
}

export interface CreateBotGatewayReq {
    gatewayUrl: string;
}
