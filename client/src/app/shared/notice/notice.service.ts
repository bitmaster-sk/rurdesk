import { PlatformLocation } from '@angular/common';
import { Injectable } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { Message } from 'src/app/message/model/message.model';
import { Notification } from 'src/app/notification/model/notification.model';
import { ReadIssueRelationDto } from 'src/app/issue/model/issue-relation.model';
import { Issue } from 'src/app/issue/model/issue.model';
import { UserService } from 'src/app/auth/user.service';
import { IssueParticipantModel } from 'src/app/issue/model/issue-participant.model';
import { NoticeSubject } from './constant/notice-subject.enum';
import { NoticeAction } from './constant/notice-action.enum';
import { Notice } from './model/notice.model';

@Injectable({
    providedIn: 'root'
})
export class NoticeService {
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

    constructor(
        private location: PlatformLocation,
        private sUser: UserService
    ) {
        if (!this.socket) {
            this.openSocket();
        }
    }

    private openSocket(): void {
        try {
            // Anonymous visitors (login page) have no token — connecting would be a
            // guaranteed 401, so wait and retry until a session exists.
            const token = this.sUser.getAuthLocal();
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

    private onOpen(evt: Event): void {}

    private onError(evt: Event): void {
        console.error(evt);
    }

    private onClose(evt: CloseEvent): void {
        this.openSocketWithDelay();
    }

    private onMessage(evt: MessageEvent): void {
        try {
            const notice = JSON.parse(evt.data);
            this.dispatch(notice);
        } catch (error) {
            console.error(error);
        }
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
