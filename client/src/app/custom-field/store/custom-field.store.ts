import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { CustomField } from '../model/custom-field.model';
import { CustomFieldApi } from '../api/custom-field.api.service';

@Injectable({
    providedIn: 'root'
})
export class CustomFieldStore {
    private readonly api = inject(CustomFieldApi);

    private readonly fields = new BehaviorSubject<CustomField[] | null>(null);

    public readonly customFields$ = this.fields
        .asObservable()
        .pipe(filter((fields): fields is CustomField[] => !!fields));

    private isLoading = false;

    public constructor() {
        inject(NoticeService).customField$.subscribe(notice =>
            this.applyNotice({ action: notice.action, payload: notice.payload })
        );
    }

    public load(): void {
        this.isLoading = true;
        this.api.load$().subscribe(fields => {
            this.isLoading = false;
            this.fields.next(fields);
        });
    }

    /** For views that only read: without it they wait forever on a stream nobody loaded. */
    public ensureLoaded(): void {
        if (this.fields.value !== null || this.isLoading) {
            return;
        }
        this.load();
    }

    public customFieldsByProject$(idProject: number): Observable<CustomField[]> {
        return this.customFields$.pipe(
            map(fields =>
                fields
                    .filter(field => field.idProject === idProject)
                    .sort((a, b) => a.orderRank - b.orderRank)
            )
        );
    }

    public applyNotice(notice: { action: NoticeAction; payload: CustomField }): void {
        const current = this.fields.value ?? [];
        switch (notice.action) {
            case NoticeAction.Delete:
                this.fields.next(
                    current.filter(f => f.idCustomField !== notice.payload.idCustomField)
                );
                break;
            case NoticeAction.Create:
                this.fields.next([...current, notice.payload]);
                break;
            default:
                this.fields.next(
                    current.map(f =>
                        f.idCustomField === notice.payload.idCustomField ? notice.payload : f
                    )
                );
        }
    }
}
