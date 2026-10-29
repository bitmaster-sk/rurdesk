import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import { switchMap } from 'rxjs/operators';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { Project } from 'src/app/project/model/project.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WindowService } from 'src/app/shared/window/window.service';
import { DeleteMigrationUsageItem } from 'src/app/shared/components/delete-migration-dialog/delete-migration-option.model';
import { Tag } from '../../model/tag.model';
import { TagStore } from '../../store/tag.store';
import { TagColorService } from '../../service/tag-color.service';
import {
    TagFormWindowComponent,
    TagWindowData
} from '../tag-form-window/tag-form-window.component';

@Component({
    selector: 'app-project-tag',
    templateUrl: './project-tag.component.html',
    providers: [WindowService],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectTagComponent {
    private readonly i18n = inject(I18nService);
    private readonly sWindow = inject(WindowService);
    private readonly tagStore = inject(TagStore);
    private readonly tagColor = inject(TagColorService);

    public readonly project = input.required<Project>();

    protected readonly tags = toSignal(
        toObservable(this.project).pipe(
            switchMap(project => this.tagStore.tagsByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    protected readonly isDeleteDialogVisible = signal(false);
    protected readonly isDeleting = signal(false);
    protected readonly deleteTarget = signal<Tag | null>(null);

    protected readonly deleteUsageItems = computed<DeleteMigrationUsageItem[]>(() => {
        const count = this.deleteTarget()?.issueCount ?? 0;
        if (count === 0) {
            return [];
        }
        return [{ key: 'TAG.DELETE_USAGE', params: { count } }];
    });

    protected readonly hasDeleteUsage = computed(() => (this.deleteTarget()?.issueCount ?? 0) > 0);

    public constructor() {
        this.tagStore.load();
    }

    public onNewTag(): void {
        this.openForm('TAG.NEW', {
            tag: { idProject: this.project().idProject },
            defaultColor: this.tagColor.nextColor(this.tags())
        });
    }

    protected onEditTag(tag: Tag): void {
        this.openForm('TAG.EDIT', { tag, defaultColor: tag.color });
    }

    protected onDeleteTag(tag: Tag): void {
        this.deleteTarget.set(tag);
        this.isDeleteDialogVisible.set(true);
    }

    protected onConfirmDelete(): void {
        const target = this.deleteTarget();
        if (!target) {
            return;
        }
        this.isDeleting.set(true);
        this.tagStore.delete$(target).subscribe({
            next: () => {
                this.isDeleting.set(false);
                this.isDeleteDialogVisible.set(false);
            },
            error: () => this.isDeleting.set(false)
        });
    }

    protected onReorder(evt: CdkDragDrop<Tag[]>): void {
        const reordered = [...this.tags()];
        moveItemInArray(reordered, evt.previousIndex, evt.currentIndex);
        const renumbered = reordered.map((tag, index) => ({ ...tag, orderRank: index + 1 }));
        this.tagStore.reorder(renumbered, renumbered[evt.currentIndex]);
    }

    protected onRowPointerDown(event: PointerEvent): void {
        this.setRowWidths(event.currentTarget as HTMLElement, true);
    }

    protected onRowPointerUp(event: PointerEvent): void {
        this.setRowWidths(event.currentTarget as HTMLElement, false);
    }

    protected onDragEnded(event: CdkDragEnd): void {
        this.setRowWidths(event.source.element.nativeElement, false);
    }

    private openForm(headerKey: string, data: TagWindowData): void {
        this.sWindow
            .open<Tag | null, TagWindowData>(TagFormWindowComponent, {
                header: this.i18n.instant(headerKey),
                data
            })
            .onClose.subscribe();
    }

    private setRowWidths(row: HTMLElement, snapshot: boolean): void {
        for (const cell of Array.from(row.children)) {
            const el = cell as HTMLElement;
            el.style.width = snapshot ? `${el.offsetWidth}px` : '';
        }
    }
}
