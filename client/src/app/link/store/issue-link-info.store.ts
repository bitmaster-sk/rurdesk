import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Issue } from 'src/app/issue/model/issue.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueLinkApi } from '../api/issue-link.api.service';
import { IssueLinkConverter } from '../converter/issue-link.converter';
import { IssueLinkView } from '../entity/issue-link-view.entity';
import {
    IssueLinkInfo,
    IssueLinkIssue,
    IssueLinkRef,
    IssueLinkUser
} from '../model/issue-link.model';

@Injectable({ providedIn: 'root' })
export class IssueLinkInfoStore {
    private readonly api = inject(IssueLinkApi);
    private readonly issues = signal<ReadonlyMap<string, IssueLinkIssue>>(new Map());
    private readonly projectNames = signal<ReadonlyMap<number, string>>(new Map());
    private readonly states = signal<ReadonlyMap<number, IssueState>>(new Map());
    private readonly severities = signal<ReadonlyMap<number, IssueSeverity>>(new Map());
    private readonly issueTypes = signal<ReadonlyMap<number, IssueType>>(new Map());
    private readonly users = signal<ReadonlyMap<number, IssueLinkUser>>(new Map());
    private readonly keysRequested = new Set<string>();
    private pending: IssueLinkRef[] = [];

    public constructor() {
        inject(NoticeService)
            .issue$.pipe(takeUntilDestroyed(inject(DestroyRef)))
            .subscribe(notice => this.onIssueNotice(notice));
    }

    public view(ref: IssueLinkRef): IssueLinkView | null {
        const issue = this.issues().get(IssueLinkConverter.toKey(ref));
        if (!issue) {
            return null;
        }
        const state = issue.idState === null ? undefined : this.states().get(issue.idState);
        const issueType =
            issue.idIssueType === null ? undefined : this.issueTypes().get(issue.idIssueType);
        return {
            idProject: issue.idProject,
            idIssuePublic: issue.idIssuePublic,
            title: issue.title,
            projectName: this.projectNames().get(issue.idProject) ?? '',
            state: state ?? null,
            isFinal: state?.final ?? false,
            severity:
                issue.idSeverity === null
                    ? null
                    : (this.severities().get(issue.idSeverity) ?? null),
            issueType: issueType ?? null,
            assignee:
                issue.assignedTo === null ? null : (this.users().get(issue.assignedTo) ?? null)
        };
    }

    public request(refs: IssueLinkRef[]): void {
        const missing: IssueLinkRef[] = [];
        for (const ref of refs) {
            const key = IssueLinkConverter.toKey(ref);
            if (!this.keysRequested.has(key)) {
                this.keysRequested.add(key);
                missing.push({ idProject: ref.idProject, idIssuePublic: ref.idIssuePublic });
            }
        }
        if (missing.length === 0) {
            return;
        }
        if (this.pending.length === 0) {
            queueMicrotask(() => this.flush());
        }
        this.pending.push(...missing);
    }

    private flush(): void {
        const refs = this.pending;
        this.pending = [];
        for (let start = 0; start < refs.length; start += 100) {
            const chunk = refs.slice(start, start + 100);
            this.api.loadInfo$(chunk).subscribe({
                next: info => this.merge(info),
                error: () => {
                    for (const ref of chunk) {
                        this.keysRequested.delete(IssueLinkConverter.toKey(ref));
                    }
                }
            });
        }
    }

    private merge(info: IssueLinkInfo): void {
        this.issues.update(map => {
            const next = new Map(map);
            for (const issue of info.issues) {
                next.set(IssueLinkConverter.toKey(issue), issue);
            }
            return next;
        });
        this.projectNames.update(map => {
            const next = new Map(map);
            for (const project of info.projects) {
                next.set(project.idProject, project.name);
            }
            return next;
        });
        this.states.update(map => {
            const next = new Map(map);
            for (const state of info.states) {
                next.set(state.idState, state);
            }
            return next;
        });
        this.severities.update(map => {
            const next = new Map(map);
            for (const severity of info.severities) {
                next.set(severity.idSeverity, severity);
            }
            return next;
        });
        this.issueTypes.update(map => {
            const next = new Map(map);
            for (const issueType of info.issueTypes) {
                next.set(issueType.idIssueType, issueType);
            }
            return next;
        });
        this.users.update(map => {
            const next = new Map(map);
            for (const user of info.users) {
                next.set(user.idUser, user);
            }
            return next;
        });
    }

    private onIssueNotice(notice: Notice<Issue>): void {
        const issue = notice.payload;
        if (typeof issue !== 'object' || issue === null) {
            return;
        }
        const key = IssueLinkConverter.toKey(issue);
        if (!this.issues().has(key)) {
            return;
        }
        if (notice.action === NoticeAction.Delete) {
            this.issues.update(map => {
                const next = new Map(map);
                next.delete(key);
                return next;
            });
            return;
        }
        if (notice.action === NoticeAction.Update) {
            this.issues.update(map =>
                new Map(map).set(key, {
                    idProject: issue.idProject,
                    idIssuePublic: issue.idIssuePublic,
                    title: issue.title,
                    idState: issue.idState,
                    idSeverity: issue.idSeverity,
                    idIssueType: issue.idIssueType,
                    assignedTo: issue.assignedTo ?? null
                })
            );
        }
    }
}
