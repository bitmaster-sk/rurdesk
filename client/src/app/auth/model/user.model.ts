export interface User {
    idUser: number;
    name: string;
    email: string;
    colorAvatarBg: string;
    avatarInitials?: string | null;
    isAgent?: boolean;
    isAdmin?: boolean;
}
