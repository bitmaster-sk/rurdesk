export function authHeaders(token: string): Record<string, string> {
    return { Authorization: `Bearer ${token}` };
}
