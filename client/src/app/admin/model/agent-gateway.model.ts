export interface AgentGateway {
    idGateway: number;
    idUserAgent: number;
    gatewayUrl: string;
    configJson: string;
    createdAt: string;
}

export interface CreateAgentGatewayRes extends AgentGateway {
    trackerToGatewayToken: string;
}

export interface CreateAgentGatewayReq {
    gatewayUrl: string;
}
