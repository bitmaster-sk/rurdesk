import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { combineLatest } from 'rxjs';
import { filter, first } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { MessageService } from 'src/app/message/message.service';
import { Message } from 'src/app/message/model/message.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { isToday, isYesterday, format } from 'date-fns';
import {
    CommentTimelineItem,
    TimelineDisplayItem,
    TimelineItem,
    TimelineItemType
} from '../../entity/timeline-item.entity';
import { UserService } from 'src/app/auth/user.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { I18nService } from 'src/app/shared/i18n/i18n.service';

@Component({
    selector: 'app-issue-activity-feed',
    templateUrl: './issue-activity-feed.component.html',
    styleUrls: ['./issue-activity-feed.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueActivityFeedComponent implements AfterViewInit {
    public readonly idIssue = input.required<number>();
    public readonly idProject = input.required<number>();
    public readonly pendingTrack = input<Track | null>(null);
    public readonly agentRun = input<AgentRun | null>(null);

    public readonly approveAgentRun = output<void>();
    public readonly approveAgentRunMockup = output<string>();

    private readonly scrollContainer = viewChild<ElementRef<HTMLDivElement>>('scrollContainer');

    private readonly i18n = inject(I18nService);

    private readonly sMessage = inject(MessageService);
    private readonly sTracker = inject(TrackerService);
    private readonly sNotice = inject(NoticeService);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly sUser = inject(UserService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly currentUserId = this.sUser.getUser().idUser;

    private readonly allItems = signal<TimelineItem[]>([]);
    private readonly liveItems = signal<TimelineItem[]>([]);
    protected readonly usersMap = signal<Map<number, User>>(new Map());

    public readonly activeFilters = signal<Set<TimelineItemType>>(new Set());
    public readonly idMessageEdit = signal<number | null>(null);

    public readonly displayItems = computed(() =>
        [...this.allItems(), ...this.liveItems()].sort(
            (a, b) => a.date.getTime() - b.date.getTime()
        )
    );

    public readonly filteredItems = computed(() => {
        const filters = this.activeFilters();
        const items = this.displayItems();
        return filters.size === 0 ? items : items.filter(i => filters.has(i.type));
    });

    public readonly anchorTarget = signal<{
        idParentMessage: number;
        lineStart: number;
        lineEnd: number;
    } | null>(null);

    // idMessage of the most recent plan-kind comment in the thread. Used by the
    // template to scope the Approve button to a single comment even after a
    // revision adds another plan message — without this, every plan-prefixed
    // bot message in history would render Approve while the run is awaiting
    // approval.
    public readonly idLatestPlanMessage = computed<number | null>(() => {
        let latestId: number | null = null;
        let latestTime = -Infinity;
        for (const item of this.displayItems()) {
            if (item.type !== 'comment') continue;
            const msg = item.data as Message;
            if (
                msg.messageKind !== MessageKind.Design &&
                msg.messageKind !== MessageKind.ImplementationPlan
            )
                continue;
            const t = msg.createdAt.getTime();
            if (t > latestTime && msg.idMessage != null) {
                latestTime = t;
                latestId = msg.idMessage;
            }
        }
        return latestId;
    });

    public readonly mentionCandidates = computed(() => Array.from(this.usersMap().values()));

    public readonly childrenByParent = computed<Map<number, Message[]>>(() => {
        const result = new Map<number, Message[]>();
        for (const item of this.displayItems()) {
            if (item.type === 'comment') {
                const msg = item.data as Message;
                if (msg.anchor) {
                    const parentId = msg.anchor.idParentMessage;
                    const existing = result.get(parentId);
                    if (existing) {
                        existing.push(msg);
                    } else {
                        result.set(parentId, [msg]);
                    }
                }
            }
        }
        return result;
    });

    public readonly displayGroups = computed<TimelineDisplayItem[]>(() => {
        const items = this.filteredItems();
        return items.map((item, i) => {
            const label = this.getDateLabel(item.date);
            const prevLabel = i > 0 ? this.getDateLabel(items[i - 1].date) : null;
            return {
                item,
                showSeparator: i === 0 || label !== prevLabel,
                dateLabel: label
            };
        });
    });

    private hasScrolled = false;

    public constructor() {
        effect(() => {
            if (this.allItems().length > 0 && !this.hasScrolled) {
                this.hasScrolled = true;
                this.scrollToBottom();
            }
        });

        effect(() => {
            const track = this.pendingTrack();
            if (track == null || !track.tracked || !track.endAt) return;
            const endAt = track.endAt;
            this.liveItems.update(items => [
                ...items,
                { type: 'time', date: new Date(endAt), data: track }
            ]);
            this.scrollToBottom();
        });
    }

    public ngAfterViewInit(): void {
        this.projectMemberStore.load(this.idProject());

        const usersMap$ = this.projectMemberStore.usersMap$;
        const messages$ = this.sMessage.loadMessages(this.idIssue(), MessageRecipientType.issue);
        const tracks$ = this.sTracker.loadTracks({ idIssue: this.idIssue() });

        combineLatest([messages$, tracks$, usersMap$])
            .pipe(first())
            .subscribe(([messages, tracks, usersMap]) => {
                this.usersMap.set(usersMap);
                this.allItems.set(this.mergeItems(messages, tracks));
            });

        this.sNotice.Message.pipe(
            filter(
                notice =>
                    notice.payload.idMessageRecipientType === MessageRecipientType.issue &&
                    notice.payload.idRecipient === this.idIssue()
            ),
            takeUntilDestroyed(this.destroyRef)
        ).subscribe(notice => {
            if (notice.action === NoticeAction.Update) {
                this.updateComment({
                    ...notice.payload,
                    createdAt: new Date(notice.payload.createdAt)
                });
                return;
            }
            const msg = notice.payload;
            const item: CommentTimelineItem = {
                type: 'comment',
                date: new Date(msg.createdAt),
                data: { ...msg, createdAt: new Date(msg.createdAt) }
            };
            this.liveItems.update(items => [...items, item]);
            this.scrollToBottom();
        });
    }

    public setFilter(type: 'all' | TimelineItemType): void {
        if (type === 'all') {
            this.activeFilters.set(new Set());
            return;
        }
        this.activeFilters.update(filters => {
            const next = new Set(filters);
            if (next.has(type)) {
                next.delete(type);
            } else {
                next.add(type);
            }
            if (next.size === 0) {
                return new Set();
            }
            return next;
        });
    }

    public onMessage(text: string): void {
        const anchor = this.anchorTarget();
        const anchorParam = anchor
            ? {
                  idParentMessage: anchor.idParentMessage,
                  anchorLineStart: anchor.lineStart,
                  anchorLineEnd: anchor.lineEnd
              }
            : undefined;
        this.sMessage
            .insertMessage(this.idIssue(), MessageRecipientType.issue, text, anchorParam)
            .subscribe(msg => {
                msg.isRead = true;
                const item: CommentTimelineItem = {
                    type: 'comment',
                    date: new Date(msg.createdAt),
                    data: msg
                };
                this.liveItems.update(items => [...items, item]);
                this.anchorTarget.set(null);
                this.scrollToBottom();
            });
    }

    public onAddAnchor(
        parentMessage: Message,
        event: { lineStart: number; lineEnd: number }
    ): void {
        this.anchorTarget.set({
            idParentMessage: parentMessage.idMessage,
            lineStart: event.lineStart,
            lineEnd: event.lineEnd
        });
    }

    public onCancelAnchor(): void {
        this.anchorTarget.set(null);
    }

    public onEditRequest(message: Message): void {
        this.idMessageEdit.set(message.idMessage);
    }

    public onEditCancel(): void {
        this.idMessageEdit.set(null);
    }

    public onEditSave(message: Message, newText: string): void {
        this.sMessage.updateMessage(message.idMessage, newText).subscribe(updated => {
            this.updateComment(updated);
            this.idMessageEdit.set(null);
        });
    }

    public getDateLabel(date: Date): string {
        if (isToday(date)) return this.i18n.instant('DATE.TODAY');
        if (isYesterday(date)) return this.i18n.instant('DATE.YESTERDAY');
        return date.getFullYear() === new Date().getFullYear()
            ? format(date, 'MMM d')
            : format(date, 'MMM d, yyyy');
    }

    public getUserForTrack(track: Track): User | undefined {
        if (track.idUser == null) return undefined;
        return this.usersMap().get(track.idUser);
    }

    private scrollToBottom(): void {
        setTimeout(() => {
            const el = this.scrollContainer()?.nativeElement;
            if (el) el.scrollTop = el.scrollHeight;
        }, 0);
    }

    private updateComment(updated: Message): void {
        const applyUpdate = (items: TimelineItem[]): TimelineItem[] =>
            items.map(item =>
                item.type === 'comment' && item.data.idMessage === updated.idMessage
                    ? {
                          ...item,
                          data: {
                              ...item.data,
                              message: updated.message,
                              updatedAt: updated.updatedAt
                          }
                      }
                    : item
            );
        this.allItems.update(applyUpdate);
        this.liveItems.update(applyUpdate);
    }

    private mergeItems(messages: Message[], tracks: Track[]): TimelineItem[] {
        const commentItems: TimelineItem[] = messages.map(msg => ({
            type: 'comment' as const,
            date: new Date(msg.createdAt),
            data: msg
        }));

        const timeItems: TimelineItem[] = tracks
            .filter((t): t is Track & { endAt: Date } => (t.tracked ?? 0) > 0 && !!t.endAt)
            .map(t => ({
                type: 'time' as const,
                date: new Date(t.endAt),
                data: t
            }));

        return [...commentItems, ...timeItems].sort((a, b) => a.date.getTime() - b.date.getTime());
    }
}
