import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    OnDestroy,
    OnInit,
    ViewEncapsulation,
    computed,
    inject,
    input,
    signal
} from '@angular/core';

export enum SplitCollapsed {
    None = 'none',
    Start = 'start',
    End = 'end'
}

interface SplitPaneState {
    ratio: number;
    collapsed: SplitCollapsed;
}

@Component({
    selector: 'ui-split-pane',
    standalone: false,
    templateUrl: './split-pane.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        '[class.ui-split-pane--collapsed-start]': 'collapsed() === Collapsed.Start',
        '[class.ui-split-pane--collapsed-end]': 'collapsed() === Collapsed.End',
        '[class.ui-split-pane--dragging]': 'isDragging()',
        '[style.--ui-split-pane-min.px]': 'minPx()'
    }
})
export class UiSplitPaneComponent implements OnInit, OnDestroy {
    private static readonly STEP_PERCENT = 5;
    private static readonly FINE_STEP_PERCENT = 1;

    // Ordered left→right, so a collapse step is just a move within this array.
    private static readonly POSITIONS: SplitCollapsed[] = [
        SplitCollapsed.Start,
        SplitCollapsed.None,
        SplitCollapsed.End
    ];

    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    private readonly resizeObserver = new ResizeObserver(() => this.setRatio(this.ratio(), false));
    private grabOffset = 0;

    public readonly storageKey = input<string | null>(null);
    public readonly minPx = input(280);
    public readonly defaultRatio = input(50);
    public readonly label = input('Panel width');
    public readonly collapseStartLabel = input('Collapse left panel');
    public readonly collapseEndLabel = input('Collapse right panel');
    public readonly restoreLabel = input('Restore split');
    public readonly resetLabel = input('Reset to an even split');
    public readonly alreadyDefaultLabel = input('Already at the even split');

    public readonly ratio = signal(50);
    public readonly collapsed = signal(SplitCollapsed.None);
    public readonly isDragging = signal(false);
    public readonly isAtLimit = signal(false);

    public readonly Collapsed = SplitCollapsed;

    public readonly isStartCollapsed = computed(() => this.collapsed() === SplitCollapsed.Start);
    public readonly isEndCollapsed = computed(() => this.collapsed() === SplitCollapsed.End);

    public readonly isDefault = computed(
        () =>
            this.collapsed() === SplitCollapsed.None &&
            Math.abs(this.ratio() - this.defaultRatio()) < 0.5
    );

    // Inline flex-basis beats the stylesheet, so the collapsed edges have to be
    // folded in here — a CSS-only collapse rule would never win.
    public readonly startBasis = computed(() => {
        if (this.isStartCollapsed()) {
            return 0;
        }
        if (this.isEndCollapsed()) {
            return 100;
        }
        return this.ratio();
    });

    public readonly ariaValueNow = computed(() => {
        if (this.isStartCollapsed()) {
            return 0;
        }
        if (this.isEndCollapsed()) {
            return 100;
        }
        return Math.round(this.ratio());
    });

    public readonly startLabel = computed(() =>
        this.isEndCollapsed() ? this.restoreLabel() : this.collapseStartLabel()
    );

    public readonly endLabel = computed(() =>
        this.isStartCollapsed() ? this.restoreLabel() : this.collapseEndLabel()
    );

    public readonly resetTooltip = computed(() =>
        this.isDefault() ? this.alreadyDefaultLabel() : this.resetLabel()
    );

    public ngOnInit(): void {
        const restored = this.readState();
        this.collapsed.set(restored?.collapsed ?? SplitCollapsed.None);
        this.setRatio(restored?.ratio ?? this.defaultRatio(), false);
        // Observes the host, not the window: the pane also narrows when a sidebar
        // toggles, which never fires a window resize.
        this.resizeObserver.observe(this.host.nativeElement);
    }

    public ngOnDestroy(): void {
        this.resizeObserver.disconnect();
    }

    public onPointerDown(event: PointerEvent): void {
        const target = event.target as HTMLElement;
        if (target.closest('.ui-split-pane__btn')) {
            return;
        }
        const splitter = target.closest('.ui-split-pane__splitter');
        const splitterRect = splitter?.getBoundingClientRect();
        this.grabOffset = splitterRect
            ? event.clientX - (splitterRect.left + splitterRect.width / 2)
            : 0;
        this.collapsed.set(SplitCollapsed.None);
        this.isDragging.set(true);
        target.setPointerCapture(event.pointerId);
        event.preventDefault();
    }

    public onPointerMove(event: PointerEvent): void {
        if (!this.isDragging()) {
            return;
        }
        const rect = this.host.nativeElement.getBoundingClientRect();
        this.setRatio(((event.clientX - this.grabOffset - rect.left) / rect.width) * 100, false);
    }

    public onPointerUp(): void {
        if (!this.isDragging()) {
            return;
        }
        this.isDragging.set(false);
        this.isAtLimit.set(false);
        this.persist();
    }

    public onDoubleClick(event: MouseEvent): void {
        if ((event.target as HTMLElement).closest('.ui-split-pane__btn')) {
            return;
        }
        this.reset();
    }

    public onStep(delta: number): void {
        const index = UiSplitPaneComponent.POSITIONS.indexOf(this.collapsed()) + delta;
        if (index < 0 || index >= UiSplitPaneComponent.POSITIONS.length) {
            return;
        }
        this.collapsed.set(UiSplitPaneComponent.POSITIONS[index]);
        this.persist();
    }

    public onReset(): void {
        if (this.isDefault()) {
            return;
        }
        this.reset();
    }

    public onKeydown(event: KeyboardEvent): void {
        const step = event.shiftKey
            ? UiSplitPaneComponent.FINE_STEP_PERCENT
            : UiSplitPaneComponent.STEP_PERCENT;

        switch (event.key) {
            case 'ArrowLeft':
                this.nudge(-step);
                break;
            case 'ArrowRight':
                this.nudge(step);
                break;
            case 'Home':
                this.collapse(SplitCollapsed.Start);
                break;
            case 'End':
                this.collapse(SplitCollapsed.End);
                break;
            case 'Enter':
            case ' ':
                this.reset();
                break;
            default:
                return;
        }
        event.preventDefault();
    }

    private nudge(delta: number): void {
        const collapsed = this.collapsed();
        if (
            (collapsed === SplitCollapsed.Start && delta < 0) ||
            (collapsed === SplitCollapsed.End && delta > 0)
        ) {
            return;
        }
        if (collapsed !== SplitCollapsed.None) {
            this.onStep(delta < 0 ? -1 : 1);
            return;
        }
        this.setRatio(this.ratio() + delta, true);
    }

    private collapse(side: SplitCollapsed): void {
        this.collapsed.set(side);
        this.persist();
    }

    private reset(): void {
        this.collapsed.set(SplitCollapsed.None);
        this.setRatio(this.defaultRatio(), true);
    }

    private setRatio(next: number, persist: boolean): void {
        const { min, max } = this.limits();
        const clamped = Math.min(Math.max(next, min), max);
        this.isAtLimit.set(this.isDragging() && Math.abs(clamped - next) > 0.01);
        this.ratio.set(clamped);
        if (persist) {
            this.persist();
        }
    }

    private limits(): { min: number; max: number } {
        const total = this.host.nativeElement.clientWidth;
        if (total <= 0) {
            return { min: 0, max: 100 };
        }
        const min = Math.min((this.minPx() / total) * 100, 50);
        return { min, max: 100 - min };
    }

    private persist(): void {
        const key = this.storageKey();
        if (!key) {
            return;
        }
        const state: SplitPaneState = { ratio: this.ratio(), collapsed: this.collapsed() };
        try {
            localStorage.setItem(key, JSON.stringify(state));
        } catch {
            // Private mode / quota — the split simply does not survive a reload.
        }
    }

    private readState(): SplitPaneState | null {
        const key = this.storageKey();
        if (!key) {
            return null;
        }
        try {
            const raw = localStorage.getItem(key);
            if (!raw) {
                return null;
            }
            const parsed: unknown = JSON.parse(raw);
            return this.toState(parsed);
        } catch {
            return null;
        }
    }

    private toState(parsed: unknown): SplitPaneState | null {
        if (typeof parsed !== 'object' || parsed === null) {
            return null;
        }
        const candidate = parsed as Partial<SplitPaneState>;
        const ratio = Number(candidate.ratio);
        const collapsed = UiSplitPaneComponent.POSITIONS.includes(
            candidate.collapsed as SplitCollapsed
        )
            ? (candidate.collapsed as SplitCollapsed)
            : SplitCollapsed.None;
        // The two halves are parsed independently so a bad ratio does not also
        // discard a legitimately collapsed pane.
        return {
            ratio: Number.isFinite(ratio) && ratio > 0 && ratio < 100 ? ratio : this.defaultRatio(),
            collapsed
        };
    }
}
