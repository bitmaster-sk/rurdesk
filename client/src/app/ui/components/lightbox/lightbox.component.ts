import {
    ChangeDetectionStrategy,
    Component,
    HostListener,
    computed,
    effect,
    input,
    model,
    output,
    signal
} from '@angular/core';
import { UiLightboxZoom, ZoomState } from './lightbox-zoom';

@Component({
    selector: 'ui-lightbox',
    templateUrl: './lightbox.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class UiLightboxComponent {
    public readonly visible = model(false);
    public readonly header = input('');
    public readonly src = input<string | null>(null);
    public readonly alt = input('');
    public readonly hasMany = input(false);
    public readonly previous = output<void>();
    public readonly next = output<void>();

    protected readonly step = UiLightboxZoom.step;
    protected readonly zoom = signal<ZoomState>(UiLightboxZoom.initial);
    protected readonly isDragging = signal(false);
    private lastPointer: { x: number; y: number } | null = null;

    protected readonly isZoomed = computed(() => this.zoom().scale > UiLightboxZoom.minScale);
    protected readonly isMaxZoom = computed(() => this.zoom().scale >= UiLightboxZoom.maxScale);
    protected readonly zoomPercent = computed(() => Math.round(this.zoom().scale * 100));
    protected readonly transform = computed(() => UiLightboxZoom.toTransform(this.zoom()));

    public constructor() {
        effect(() => {
            this.src();
            this.resetZoom();
        });
    }

    protected zoomBy(factor: number): void {
        this.zoom.update(state => UiLightboxZoom.zoomAt(state, factor, 0, 0));
    }

    protected resetZoom(): void {
        this.zoom.set(UiLightboxZoom.initial);
        this.isDragging.set(false);
        this.lastPointer = null;
    }

    protected onWheel(event: WheelEvent): void {
        event.preventDefault();
        const { cx, cy } = this.pointFromCenter(event);
        const factor = Math.exp(-event.deltaY * UiLightboxZoom.wheelSpeed);
        this.zoom.update(state => UiLightboxZoom.zoomAt(state, factor, cx, cy));
    }

    protected onDoubleClick(event: MouseEvent): void {
        if (this.isZoomed()) {
            this.resetZoom();
            return;
        }
        const { cx, cy } = this.pointFromCenter(event);
        this.zoom.update(state => UiLightboxZoom.zoomAt(state, this.step * this.step, cx, cy));
    }

    protected onPointerDown(event: PointerEvent): void {
        if (!this.isZoomed()) {
            return;
        }
        (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
        this.lastPointer = { x: event.clientX, y: event.clientY };
        this.isDragging.set(true);
    }

    protected onPointerMove(event: PointerEvent): void {
        if (!this.lastPointer) {
            return;
        }
        const dx = event.clientX - this.lastPointer.x;
        const dy = event.clientY - this.lastPointer.y;
        this.lastPointer = { x: event.clientX, y: event.clientY };
        this.zoom.update(state => UiLightboxZoom.panBy(state, dx, dy));
    }

    protected onPointerUp(): void {
        this.lastPointer = null;
        this.isDragging.set(false);
    }

    @HostListener('document:keydown', ['$event'])
    protected onKeydown(event: KeyboardEvent): void {
        if (!this.visible()) {
            return;
        }
        switch (event.key) {
            case 'ArrowRight':
                this.next.emit();
                break;
            case 'ArrowLeft':
                this.previous.emit();
                break;
            case '+':
            case '=':
                this.zoomBy(this.step);
                break;
            case '-':
                this.zoomBy(1 / this.step);
                break;
            case '0':
                this.resetZoom();
                break;
        }
    }

    private pointFromCenter(event: MouseEvent): { cx: number; cy: number } {
        const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
        return {
            cx: event.clientX - (rect.left + rect.width / 2),
            cy: event.clientY - (rect.top + rect.height / 2)
        };
    }
}
