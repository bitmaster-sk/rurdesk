import { ChangeDetectionStrategy, Component, inject, output } from '@angular/core';
import { IssueLinkOption } from '../../model/issue-link.model';
import { IssueLinkPicker } from '../../service/issue-link-picker.service';

@Component({
    selector: 'app-issue-link-picker',
    templateUrl: './issue-link-picker.component.html',
    styleUrls: ['./issue-link-picker.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueLinkPickerComponent {
    protected readonly picker = inject(IssueLinkPicker);

    public readonly picked = output<IssueLinkOption>();

    protected onPointerMove(event: PointerEvent, index: number): void {
        // Chrome fires pointermove without movement when the list changes under a still pointer.
        if (event.movementX !== 0 || event.movementY !== 0) {
            this.picker.activeIndex.set(index);
        }
    }
}
