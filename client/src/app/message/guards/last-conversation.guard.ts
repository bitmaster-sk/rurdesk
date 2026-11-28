import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ProjectContextStore } from 'src/app/home/store/project-context.store';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';

const LAST_CONVERSATION_KEY = 'message.lastConversation';
const CONVERSATION_URL = /^\/message\/\d+\/\d+\/view$/;

export abstract class LastConversationGuard {
    /**
     * `/message` alone opens a chat: the one opened last, else the chat of the side menu's project,
     * else the first project chat, else My page.
     */
    public static readonly canActivate: CanActivateFn = (): UrlTree | Observable<UrlTree> => {
        const router = inject(Router);
        const saved = LastConversationGuard.read();
        if (saved !== null) {
            return router.parseUrl(saved);
        }
        const idProject = inject(ProjectContextStore).idProject();
        if (idProject !== null) {
            return router.parseUrl(LastConversationGuard.projectChat(idProject));
        }
        return inject(ProjectApi)
            .load$()
            .pipe(
                map(projects =>
                    router.parseUrl(
                        projects.length > 0
                            ? LastConversationGuard.projectChat(projects[0].idProject)
                            : '/'
                    )
                )
            );
    };

    /** Remembers the chat the user opened, for the next `/message`. */
    public static save(url: string): void {
        try {
            localStorage.setItem(LAST_CONVERSATION_KEY, url);
        } catch {
            /* best-effort: /message then falls back to the project chat */
        }
    }

    /** Drops the remembered chat when it is `url`: the user can no longer open it. */
    public static forget(url: string): void {
        try {
            if (localStorage.getItem(LAST_CONVERSATION_KEY) === url) {
                localStorage.removeItem(LAST_CONVERSATION_KEY);
            }
        } catch {
            /* best-effort */
        }
    }

    /** Drops the remembered chat on logout: it belongs to the user who leaves. */
    public static clear(): void {
        try {
            localStorage.removeItem(LAST_CONVERSATION_KEY);
        } catch {
            /* best-effort */
        }
    }

    private static read(): string | null {
        try {
            const saved = localStorage.getItem(LAST_CONVERSATION_KEY);
            return saved !== null && CONVERSATION_URL.test(saved) ? saved : null;
        } catch {
            return null;
        }
    }

    private static projectChat(idProject: number): string {
        return `/message/${idProject}/${MessageRecipientType.project}/view`;
    }
}
