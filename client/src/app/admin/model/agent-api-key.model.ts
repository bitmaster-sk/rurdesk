export interface AgentApiKey {
    idApiKey: number;
    idUser: number;
    name: string;
    rateLimitOverride: number | null;
    createdAt: string;
    expiresAt: string | null;
    lastUsedAt: string | null;
}

export interface CreateAgentKeyRes extends AgentApiKey {
    rawKey: string;
}
