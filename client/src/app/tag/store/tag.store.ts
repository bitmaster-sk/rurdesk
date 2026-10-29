import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable, Subscription } from 'rxjs';
import { filter, map, tap } from 'rxjs/operators';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TagApi } from '../api/tag.api.service';
import { CreateTagReq, EditTagReq, Tag } from '../model/tag.model';

@Injectable({
    providedIn: 'root'
})
export class TagStore {
    private readonly api = inject(TagApi);

    private readonly tags = new BehaviorSubject<Tag[] | null>(null);

    private readonly deletedTagIds = new Set<number>();

    private loading?: Subscription;

    public readonly tags$ = this.tags.asObservable().pipe(filter((tags): tags is Tag[] => !!tags));

    public readonly tagsMap$: Observable<Map<number, Tag>> = this.tags$.pipe(
        map(tags => new Map(tags.map(tag => [tag.idTag, tag])))
    );

    public constructor() {
        inject(NoticeService).tag$.subscribe(notice => {
            this.applyNotice({ action: notice.action, payload: notice.payload });
            // A move or delete renumbers every tag of the project, the notice carries only one.
            if (notice.action !== NoticeAction.Create) {
                this.load();
            }
        });
    }

    public load(): void {
        this.loading?.unsubscribe();
        this.loading = this.api.load$().subscribe({
            next: tags => this.tags.next(tags),
            error: () => (this.loading = undefined)
        });
    }

    public tagsByProject$(idProject: number): Observable<Tag[]> {
        return this.tags$.pipe(
            map(tags =>
                tags
                    .filter(tag => tag.idProject === idProject)
                    .sort((a, b) => a.orderRank - b.orderRank)
            )
        );
    }

    public knownTagIds(): Set<number> | null {
        const tags = this.tags.value;
        return tags ? new Set(tags.map(tag => tag.idTag)) : null;
    }

    public isDeleted(idTag: number): boolean {
        return this.deletedTagIds.has(idTag);
    }

    public insert$(tag: CreateTagReq): Observable<Tag> {
        return this.api
            .insert$(tag)
            .pipe(
                tap(created => this.applyNotice({ action: NoticeAction.Create, payload: created }))
            );
    }

    public update$(tag: EditTagReq): Observable<Tag> {
        return this.api
            .update$(tag)
            .pipe(
                tap(updated => this.applyNotice({ action: NoticeAction.Update, payload: updated }))
            );
    }

    public reorder(renumbered: Tag[], moved: Tag): void {
        const before = this.tags.value;
        const byId = new Map(renumbered.map(tag => [tag.idTag, tag]));
        this.tags.next((before ?? []).map(tag => byId.get(tag.idTag) ?? tag));
        this.api.update$(moved).subscribe({
            next: () => this.load(),
            error: () => this.tags.next(before)
        });
    }

    public delete$(tag: Tag): Observable<Tag> {
        return this.api
            .delete$(tag.idProject, tag.idTag)
            .pipe(tap(() => this.applyNotice({ action: NoticeAction.Delete, payload: tag })));
    }

    public applyNotice(notice: { action: NoticeAction; payload: Tag }): void {
        const current = this.tags.value ?? [];
        const others = current.filter(tag => tag.idTag !== notice.payload.idTag);
        if (notice.action === NoticeAction.Delete) {
            this.deletedTagIds.add(notice.payload.idTag);
            this.tags.next(others);
            return;
        }
        this.tags.next([...others, notice.payload]);
    }
}
