export interface AgentGateway {
    idBotGateway: number;
    idUserBot: number;
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
