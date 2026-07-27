export interface User {
    idUser: number;
    name: string;
    email: string;
    colorAvatarBg: string;
    isBot?: boolean;
    isAdmin?: boolean;
}
