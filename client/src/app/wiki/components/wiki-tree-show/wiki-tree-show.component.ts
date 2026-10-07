import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { WikiLayoutStore } from '../../store/wiki-layout.store';

@Component({
    selector: 'app-wiki-tree-show',
    templateUrl: './wiki-tree-show.component.html',
    styleUrls: ['./wiki-tree-show.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiTreeShowComponent {
    protected readonly layout = inject(WikiLayoutStore);
}
