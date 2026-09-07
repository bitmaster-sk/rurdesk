import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import {
    AgentGateway,
    CreateAgentGatewayReq,
    CreateAgentGatewayRes
} from '../model/agent-gateway.model';

@Injectable({ providedIn: 'root' })
export class AgentGatewayApi {
    private readonly http = inject(HttpClient);

    public load$(idUser: number): Observable<AgentGateway | null> {
        return this.http.get<AgentGateway | null>(this.baseUrl(idUser), {
            context: RequestContext.disableErrorToast()
        });
    }

    public insert$(idUser: number, req: CreateAgentGatewayReq): Observable<CreateAgentGatewayRes> {
        return this.http.post<CreateAgentGatewayRes>(this.baseUrl(idUser), req, {
            context: RequestContext.disableErrorToast()
        });
    }

    public update$(idUser: number, req: CreateAgentGatewayReq): Observable<AgentGateway> {
        return this.http.patch<AgentGateway>(this.baseUrl(idUser), req);
    }

    public regenerateToken$(idUser: number): Observable<CreateAgentGatewayRes> {
        return this.http.post<CreateAgentGatewayRes>(
            `${this.baseUrl(idUser)}/token`,
            {},
            { context: RequestContext.disableErrorToast() }
        );
    }

    public delete$(idUser: number): Observable<void> {
        return this.http.delete<void>(this.baseUrl(idUser), {
            context: RequestContext.disableErrorToast()
        });
    }

    private baseUrl(idUser: number): string {
        return `/api/private/admin/user/${idUser}/gateway`;
    }
}
