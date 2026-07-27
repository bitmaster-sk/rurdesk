import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { PinView } from '../entity/pin-view.entity';

@Component({
    selector: 'app-pin-view',
    templateUrl: './pin-view.component.html',
    styleUrls: ['./pin-view.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class PinViewComponent {
    public readonly pin = input.required<PinView>();
    public readonly deletePin = output<PinView>();

    public readonly pinState = computed<IssueState | undefined>(() => {
        const pin = this.pin();
        if (!pin.stateName) return undefined;
        return {
            idState: 0,
            idProject: 0,
            name: pin.stateName,
            start: pin.stateIsStart ?? false,
            final: pin.stateIsFinal ?? false,
            protected: false,
            orderRank: 0
        };
    });

    public onDeletePin(): void {
        this.deletePin.emit(this.pin());
    }
}
