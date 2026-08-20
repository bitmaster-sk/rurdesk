import { PlatformLocation } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { Message } from 'src/app/message/model/message.model';
import { Notification } from 'src/app/notification/model/notification.model';
import { ReadIssueRelationDto } from 'src/app/issue/model/issue-relation.model';
import { Issue } from 'src/app/issue/model/issue.model';
import { AuthTokenStore } from 'src/app/auth/store/auth-token.store';
import { IssueParticipantModel } from 'src/app/issue/model/issue-participant.model';
import { NoticeSubject } from './constant/notice-subject.enum';
import { NoticeAction } from './constant/notice-action.enum';
import { Notice } from './model/notice.model';

@Injectable({
    providedIn: 'root'
})
export class NoticeService {
    private readonly location = inject(PlatformLocation);
    private readonly tokenStore = inject(AuthTokenStore);

    public Message: Subject<Notice<Message>> = new Subject<Notice<Message>>();

    private notificationSubject = new Subject<Notice<Notification>>();
    public notification$: Observable<Notice<Notification>> =
        this.notificationSubject.asObservable();

    private relationSubject = new Subject<Notice<ReadIssueRelationDto[]>>();
    public relation$: Observable<Notice<ReadIssueRelationDto[]>> =
        this.relationSubject.asObservable();

    private issueSubject = new Subject<Notice<Issue>>();
    public issue$: Observable<Notice<Issue>> = this.issueSubject.asObservable();

    /** Publish an issue change locally so open views (e.g. the task detail) refresh immediately,
     *  on the same stream the WebSocket feeds — used for out-of-band mutations like command-palette
     *  actions, where the server may not echo the change back to the acting client. */
    public emitIssue(issue: Issue, action: NoticeAction = NoticeAction.Update): void {
        this.issueSubject.next({ subject: NoticeSubject.Issue, action, payload: issue });
    }

    private agentRunSubject = new Subject<Notice<unknown>>();
    public agentRun$: Observable<Notice<unknown>> = this.agentRunSubject.asObservable();

    private agentStatsSubject = new Subject<Notice<unknown>>();
    public agentStats$: Observable<Notice<unknown>> = this.agentStatsSubject.asObservable();

    private participantSubject = new Subject<
        Notice<{ idIssue: number; participants: IssueParticipantModel[] }>
    >();
    public participant$: Observable<
        Notice<{ idIssue: number; participants: IssueParticipantModel[] }>
    > = this.participantSubject.asObservable();

    private socket: WebSocket | null = null;

    private readonly RECONNECT_DELAY = 60 * 1000;

    public constructor() {
        this.openSocket();
    }

    private openSocket(): void {
        try {
            // Anonymous visitors (login page) have no token — connecting would be a
            // guaranteed 401, so wait and retry until a session exists.
            const token = this.tokenStore.getToken();
            if (!token) {
                this.openSocketWithDelay();
                return;
            }
            const protocol = this.location.protocol === 'https:' ? 'wss' : 'ws';
            // The WebSocket API cannot set an Authorization header, so the token is
            // offered as a subprotocol; the server selects the "Authorization"
            // marker to complete the handshake. Keeping localStorage as the single
            // token store avoids the cookie copy, which ignored the port and died
            // with the browser session.
            const socket = new WebSocket(
                `${protocol}://${this.location.hostname}:${this.location.port}/api/private/ws`,
                ['Authorization', token]
            );
            socket.addEventListener('open', this.onOpen.bind(this));
            socket.addEventListener('close', this.onClose.bind(this));
            socket.addEventListener('message', this.onMessage.bind(this));
            socket.addEventListener('error', this.onError.bind(this));
            this.socket = socket;
        } catch (error) {
            this.socket = null;
            console.error(error);
        }
    }

    private openSocketWithDelay(): void {
        setTimeout(() => this.openSocket(), this.RECONNECT_DELAY);
    }

    private onOpen(): void {}

    private onError(evt: Event): void {
        console.error(evt);
    }

    private onClose(): void {
        this.openSocketWithDelay();
    }

    private onMessage(evt: MessageEvent): void {
        const data: unknown = evt.data;
        if (typeof data !== 'string') {
            console.warn('unsupported notice');
            return;
        }
        try {
            const notice: unknown = JSON.parse(data);
            if (NoticeService.isNotice(notice)) {
                this.dispatch(notice);
            } else {
                console.warn('unsupported notice');
            }
        } catch (error) {
            console.error(error);
        }
    }

    private static isNotice(value: unknown): value is Notice<unknown> {
        if (value === null || typeof value !== 'object') {
            return false;
        }
        const notice = value as Record<string, unknown>;
        const subject = notice['subject'];
        const action = notice['action'];
        return (
            typeof subject === 'string' &&
            Object.values<string>(NoticeSubject).includes(subject) &&
            typeof action === 'string' &&
            Object.values<string>(NoticeAction).includes(action)
        );
    }

    private dispatch(notice: Notice<unknown>): void {
        switch (notice.subject) {
            case NoticeSubject.Message:
                this.Message.next(notice as Notice<Message>);
                break;
            case NoticeSubject.Notification:
                this.notificationSubject.next(notice as Notice<Notification>);
                break;
            case NoticeSubject.Relation:
                this.relationSubject.next(notice as Notice<ReadIssueRelationDto[]>);
                break;
            case NoticeSubject.Issue:
                this.issueSubject.next(notice as Notice<Issue>);
                break;
            case NoticeSubject.AgentRun:
                this.agentRunSubject.next(notice);
                break;
            case NoticeSubject.AgentStats:
                this.agentStatsSubject.next(notice);
                break;
            case NoticeSubject.Participant:
                this.participantSubject.next(
                    notice as Notice<{ idIssue: number; participants: IssueParticipantModel[] }>
                );
                break;
            default:
                console.warn('unsupported notice');
                break;
        }
    }
}
