export interface AgentGateway {
    idGateway: number;
    idUserAgent: number;
    gatewayUrl: string;
    createdAt: string;
    models: string[];
    modelDefault: string | null;
    modelsReportedAt: string | null;
    maxTurnsLimit: number | null;
    maxTurnsPlan: number | null;
    maxTurnsImplement: number | null;
}

export interface CreateAgentGatewayRes extends AgentGateway {
    trackerToGatewayToken: string;
}

export interface CreateAgentGatewayReq {
    gatewayUrl: string;
    maxTurnsPlan?: number | null;
    maxTurnsImplement?: number | null;
}
