import { Injectable, signal } from '@angular/core';

/**
 * Shares the sprint currently hovered during a task drag, so the dragged card's
 * preview can show a header naming the target sprint (the card itself covers the
 * tab highlight). Set by the tab strip on cdkDropListEntered/Exited, read by the
 * tile's cdkDragPreview.
 */
@Injectable({ providedIn: 'root' })
export class SprintDragService {
    public readonly hoveredName = signal<string | null>(null);
}
