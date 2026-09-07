import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import { AgentApiKey, CreateAgentKeyRes } from '../model/agent-api-key.model';

@Injectable({ providedIn: 'root' })
export class AgentApiKeyApi {
    private readonly http = inject(HttpClient);

    public load$(idUser: number): Observable<AgentApiKey | null> {
        return this.http.get<AgentApiKey | null>(this.baseUrl(idUser), {
            context: RequestContext.disableErrorToast()
        });
    }

    public insert$(idUser: number, name: string): Observable<CreateAgentKeyRes> {
        return this.http.post<CreateAgentKeyRes>(
            this.baseUrl(idUser),
            { name },
            { context: RequestContext.disableErrorToast() }
        );
    }

    public regenerate$(idUser: number): Observable<CreateAgentKeyRes> {
        return this.http.post<CreateAgentKeyRes>(
            `${this.baseUrl(idUser)}/token`,
            {},
            { context: RequestContext.disableErrorToast() }
        );
    }

    public revoke$(idUser: number): Observable<void> {
        return this.http.delete<void>(this.baseUrl(idUser), {
            context: RequestContext.disableErrorToast()
        });
    }

    private baseUrl(idUser: number): string {
        return `/api/private/admin/user/${idUser}/api-key`;
    }
}
