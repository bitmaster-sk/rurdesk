export enum Role {
    Viewer = 'viewer',
    Member = 'member',
    Owner = 'owner'
}

export const ROLE_RANK: Record<Role, number> = {
    [Role.Viewer]: 0,
    [Role.Member]: 1,
    [Role.Owner]: 2
};
