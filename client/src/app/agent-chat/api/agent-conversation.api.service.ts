import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { RequestContext } from 'src/app/core/request-context';
import { AgentThinkingRes } from 'src/app/agent/model/agent-thinking.model';
import {
    AgentConversation,
    AgentConversationAgent,
    AgentConversationDetail,
    AgentConversationListItem,
    AgentConversationMessage,
    AgentConversationSettings,
    CreateAgentConversationReq
} from '../model/agent-conversation.model';

const BASE = '/api/private/agent/conversation';

@Injectable({ providedIn: 'root' })
export class AgentConversationApi {
    private readonly http = inject(HttpClient);

    /** The owner's conversations, pinned first, then newest activity; `idProject` narrows to one project. */
    public load$(idProject?: number | null): Observable<AgentConversationListItem[]> {
        let params = new HttpParams();
        if (idProject != null) {
            params = params.set('idProject', idProject);
        }
        return this.http.get<AgentConversationListItem[]>(BASE, { params });
    }

    public insert$(req: CreateAgentConversationReq): Observable<AgentConversation> {
        return this.http.post<AgentConversation>(BASE, req);
    }

    /** Agents the owner can chat with: members of `idProject`, or of any of the owner's projects. */
    public loadAgents$(idProject: number | null): Observable<AgentConversationAgent[]> {
        let params = new HttpParams();
        if (idProject != null) {
            params = params.set('idProject', idProject);
        }
        return this.http.get<AgentConversationAgent[]>(`${BASE}/agents`, { params });
    }

    public loadUnreadCount$(): Observable<number> {
        return this.http.get<{ count: number }>(`${BASE}/unread-count`).pipe(map(res => res.count));
    }

    public loadOne$(idConversation: number): Observable<AgentConversationDetail> {
        return this.http.get<AgentConversationDetail>(`${BASE}/${idConversation}`);
    }

    public delete$(idConversation: number): Observable<void> {
        return this.http.delete<void>(`${BASE}/${idConversation}`);
    }

    public loadMessages$(idConversation: number): Observable<AgentConversationMessage[]> {
        return this.http.get<AgentConversationMessage[]>(`${BASE}/${idConversation}/message`);
    }

    public insertMessage$(
        idConversation: number,
        message: string
    ): Observable<AgentConversationMessage> {
        // The page shows the refusal above the composer instead of a toast.
        return this.http.post<AgentConversationMessage>(
            `${BASE}/${idConversation}/message`,
            { message },
            { context: RequestContext.disableErrorToast() }
        );
    }

    public insertProject$(idConversation: number, idProject: number): Observable<void> {
        // The picker shows why the project was refused instead of a toast.
        return this.http.post<void>(
            `${BASE}/${idConversation}/project`,
            { idProject },
            { context: RequestContext.disableErrorToast() }
        );
    }

    public deleteProject$(idConversation: number, idProject: number): Observable<void> {
        return this.http.delete<void>(`${BASE}/${idConversation}/project/${idProject}`);
    }

    public markRead$(idConversation: number): Observable<void> {
        return this.http.post<void>(`${BASE}/${idConversation}/read`, {});
    }

    /** Pins the conversation to the top of the list, or unpins it. */
    public setPinned$(idConversation: number, isPinned: boolean): Observable<void> {
        return this.http.put<void>(`${BASE}/${idConversation}/pin`, { isPinned });
    }

    /** Replaces the model; refused while the agent is answering. */
    public updateSettings$(
        idConversation: number,
        settings: AgentConversationSettings
    ): Observable<void> {
        return this.http.put<void>(`${BASE}/${idConversation}/settings`, settings);
    }

    /** Queues the failed turn again; the page shows a refusal itself. */
    public retry$(idConversation: number): Observable<void> {
        return this.http.post<void>(
            `${BASE}/${idConversation}/retry`,
            {},
            { context: RequestContext.disableErrorToast() }
        );
    }

    /** Asks the agent to draft one task about the given agent answer. */
    public insertTaskDraft$(idConversation: number, idMessage: number): Observable<void> {
        return this.http.post<void>(`${BASE}/${idConversation}/task-draft`, { idMessage });
    }

    /** Thinking events of one agent turn (`idTask` is the task behind a reply). */
    public loadThinking$(idConversation: number, idTask: number): Observable<AgentThinkingRes> {
        return this.http.get<AgentThinkingRes>(`${BASE}/${idConversation}/thinking`, {
            params: { idTask }
        });
    }
}
