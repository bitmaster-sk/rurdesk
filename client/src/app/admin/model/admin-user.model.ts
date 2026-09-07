export interface AdminUser {
    idUser: number;
    name: string;
    email: string;
    colorAvatarBg: string;
    isAgent: boolean;
    isAdmin: boolean;
}

export interface AdminCreateUserReq {
    name: string;
    email?: string;
    password?: string;
    isAgent: boolean;
    isAdmin?: boolean;
    colorAvatarBg?: string;
}

export interface AdminCreateUserRes extends AdminUser {
    rawKey?: string;
}

// Admin edit. Every field optional — only the present ones are applied. For an
// agent only `name` is sent (no email/admin). Reuses PATCH /admin/user/:idUser.
export interface AdminUpdateUserReq {
    name?: string;
    email?: string;
    isAdmin?: boolean;
    colorAvatarBg?: string;
}

// Emitted by the create-user dialog. For an agent we may also have created its
// gateway in the same step, so the freshly-minted tracker→gateway token rides
// along to be revealed once in the keys window.
export interface UserCreatedEvent {
    user: AdminCreateUserRes;
    gatewayToken: string | null;
}
