import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { UiDateRangeValue } from '../components/date-range-select/date-range-select.component';
import { CdkDragDrop, CdkDragEnd, moveItemInArray } from '@angular/cdk/drag-drop';
import { UiTableSortEvent } from '../components/table/table-sort.directive';
import { UiToastSeverity, UiToastService } from '../service/ui-toast.service';
import { UiBadgeSeverity } from '../components/badge/badge.component';
import { UiMenuItem } from '../components/menu/menu-item.model';
import { UiMessageSeverity } from '../components/message/message.component';
import { UiTagSeverity } from '../components/tag/tag.component';

type ButtonSeverity = 'primary' | 'secondary' | 'danger' | 'success' | 'info' | 'warn';
type ButtonVariant = 'filled' | 'outlined' | 'text';
interface GalleryMenuItem extends UiMenuItem {
    tablerIcon?: string;
}

/** Dev-only showcase of every `ui-*` primitive and its variants. */
@Component({
    selector: 'ui-gallery-page',
    standalone: false,
    templateUrl: './ui-gallery.page.html',
    styleUrl: './ui-gallery.page.scss',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class UiGalleryPage {
    protected readonly tagSeverities: UiTagSeverity[] = [
        'primary',
        'secondary',
        'success',
        'info',
        'warn',
        'danger',
        'contrast'
    ];

    protected readonly messageSeverities: UiMessageSeverity[] = [
        'info',
        'success',
        'warn',
        'danger'
    ];

    /** Control size-scale showcase rows (small / default / large). */
    protected readonly sizeScaleRows: { label: string; size: 'small' | 'large' | undefined }[] = [
        { label: 'small (26px)', size: 'small' },
        { label: 'default (32px)', size: undefined },
        { label: 'large (40px)', size: 'large' }
    ];

    protected readonly buttonSeverities: ButtonSeverity[] = [
        'primary',
        'secondary',
        'danger',
        'success',
        'info',
        'warn'
    ];

    protected readonly buttonVariants: ButtonVariant[] = ['filled', 'outlined', 'text'];

    protected readonly badgeSeverities: UiBadgeSeverity[] = [
        'primary',
        'secondary',
        'success',
        'info',
        'warn',
        'danger',
        'contrast'
    ];

    // select / multiselect / listbox demo data
    protected readonly selectOptions = [
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' },
        { label: 'Gamma — a deliberately long option label', value: 'c' },
        { label: 'Delta', value: 'd' }
    ];
    protected selectValue: string | null = 'b';
    protected selectClearValue: string | null = 'a';
    protected selectFilterValue: string | null = null;
    protected selectSmallValue: string | null = null;
    protected selectDisabledValue: string | null = 'a';

    protected readonly colorOptions = [
        { label: 'Low', value: 1, color: '#22c55e' },
        { label: 'Medium', value: 2, color: '#f59e0b' },
        { label: 'High', value: 3, color: '#ef4444' }
    ];
    protected colorValue: number | null = 2;

    protected swatchValue = '#22c55e';

    protected multiValue: string[] = ['a', 'c'];

    protected readonly listboxOptions = [
        { name: 'Ann' },
        { name: 'Bob' },
        { name: 'Cara' },
        { name: 'Dan' }
    ];
    protected lastPicked = '—';

    protected onListboxPick(option: { name: string }): void {
        this.lastPicked = option.name;
    }

    // popover demo state
    protected lastPopoverAction = '—';

    // menu demo data
    protected lastMenuAction = '—';
    protected readonly menuFlat: UiMenuItem[] = [
        { label: 'Rename', command: () => (this.lastMenuAction = 'Rename') },
        { label: 'Duplicate', command: () => (this.lastMenuAction = 'Duplicate') },
        { label: 'Archive', command: () => (this.lastMenuAction = 'Archive') }
    ];
    protected readonly menuGrouped: UiMenuItem[] = [
        {
            label: 'Projects',
            items: [
                { label: 'Alpha', icon: 'messages', badge: '3', routerLink: ['/ui'] },
                { label: 'Beta', icon: 'messages', routerLink: ['/ui'] }
            ]
        },
        { label: 'Teams (empty)', items: [] },
        {
            label: 'Direct',
            items: [{ label: 'Cara', icon: 'user', badge: '12', routerLink: ['/ui'] }]
        }
    ];
    protected readonly menuTemplate: GalleryMenuItem[] = [
        { label: 'My page', tablerIcon: 'user', routerLink: ['/ui'] },
        { label: 'Settings', tablerIcon: 'settings', routerLink: ['/ui'] },
        { separator: true },
        { label: 'Logout', tablerIcon: 'logout', routerLink: ['/ui'] }
    ];

    // checkbox / toggle / choice demo data
    protected checkboxOff = false;
    protected checkboxOn = true;

    protected toggleOff = false;
    protected toggleOn = true;

    protected toggleButtonOff = false;
    protected toggleButtonOn = true;

    protected readonly choiceOptions = [
        { label: 'List', value: 'list' },
        { label: 'Board', value: 'board' },
        { label: 'Calendar', value: 'calendar' }
    ];
    protected choiceValue: string | null = 'board';
    protected choiceEmptyValue: string | null = 'list';
    protected choiceIconValue: string | null = 'board';

    // datepicker demo data
    protected dateValue: Date | null = new Date(2026, 6, 3);
    protected datetimeValue: Date | null = new Date(2026, 6, 3, 14, 30);
    protected rangeValue: Date[] | null = null;
    protected inlineValue: Date | null = new Date(2026, 6, 3);

    // date-range-select demo data
    protected readonly dateRangePresets = [
        { label: 'Last 7 days', value: '7d' },
        { label: 'Last 30 days', value: '30d' },
        { label: 'Last 90 days', value: '90d' }
    ];
    protected dateRangeValue: UiDateRangeValue | null = null;

    // dialog demo state
    protected dialogBasicOpen = false;
    protected dialogNoHeaderOpen = false;
    protected dialogLockedOpen = false;
    protected dialogDismissableOpen = false;
    protected lastDialogAction = '—';

    // toast demo
    private readonly toastService = inject(UiToastService);
    protected showToast(severity: UiToastSeverity): void {
        // Real default life (error 4000, else 3000) so pause/progress are observable
        // during manual QA — the 60000 override belongs only in browser specs.
        this.toastService.show({
            severity,
            detail: `This is a sample ${severity} toast message.`,
            life: severity === 'error' ? 4000 : 3000
        });
    }

    // table demo data
    protected readonly tableRows = [
        { id: 1, name: 'Alpha', role: 'owner', count: 4 },
        { id: 2, name: 'Bravo', role: 'member', count: 11 },
        { id: 3, name: 'Charlie', role: 'viewer', count: 2 }
    ];
    /** Long list to show sticky header + scroll. */
    protected readonly scrollRows = Array.from({ length: 30 }, (_, i) => ({
        id: i + 1,
        name: `Row ${i + 1}`,
        value: (i * 7) % 100
    }));
    protected lastSort = '—';
    protected onGallerySort(evt: UiTableSortEvent): void {
        this.lastSort = `${evt.sortField} ${evt.sortOrder > 0 ? 'asc' : 'desc'}`;
    }

    // table reorder demo (CDK)
    protected reorderRows = [
        { id: 1, label: 'To do' },
        { id: 2, label: 'In progress' },
        { id: 3, label: 'In review' },
        { id: 4, label: 'Done' }
    ];
    protected onGalleryReorder(evt: CdkDragDrop<unknown[]>): void {
        moveItemInArray(this.reorderRows, evt.previousIndex, evt.currentIndex);
    }
    protected onGalleryRowPointerDown(event: PointerEvent): void {
        this.setGalleryRowWidths(event.currentTarget as HTMLElement, true);
    }
    protected onGalleryRowPointerUp(event: PointerEvent): void {
        this.setGalleryRowWidths(event.currentTarget as HTMLElement, false);
    }
    protected onGalleryDragEnded(event: CdkDragEnd): void {
        this.setGalleryRowWidths(event.source.element.nativeElement, false);
    }
    private setGalleryRowWidths(row: HTMLElement, snapshot: boolean): void {
        for (const cell of Array.from(row.children)) {
            const el = cell as HTMLElement;
            el.style.width = snapshot ? `${el.offsetWidth}px` : '';
        }
    }
}
