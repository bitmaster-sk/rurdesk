import { ChangeDetectionStrategy, Component, ViewChild, input, output } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';

@Component({
    selector: 'app-teammate-selector',
    templateUrl: './teammate-selector.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeammateSelectorComponent {
    @ViewChild('pop') private readonly popover!: UiPopoverComponent;

    public readonly options = input<User[]>([]);
    public readonly emptyMessage = input<string>('');
    public readonly selected = output<User>();

    public toggle(event: Event): void {
        this.popover.toggle(event);
    }

    protected onSelect(user: User): void {
        this.selected.emit(user);
        this.popover.hide();
    }
}
