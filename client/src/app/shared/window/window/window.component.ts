import { Component, Injector, Type, ViewEncapsulation, inject } from '@angular/core';
import { WindowConfig } from '../entity/window-config';
import { WindowReference } from '../window.reference';

@Component({
    selector: 'app-window',
    templateUrl: './window.component.html',
    styleUrls: ['./window.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class WindowComponent {
    /** typ komponenty zobrazenej vo vnútri okna (vytvorí ju *ngComponentOutlet) */
    public contentType!: Type<unknown>;

    /** príznak zobrazenia dialógu (dvojsmerne viazaný na <ui-dialog>) */
    public visible = true;

    /** injektor okna (WindowInjector) — cez neho content komponenta získa WindowConfig/WindowReference */
    public readonly injector = inject(Injector);

    /** konfigurácia okna (poskytnutá cez WindowInjector) */
    public config = inject(WindowConfig);

    private windowRef = inject(WindowReference);

    /** zavrie okno */
    public onClose(): void {
        this.windowRef.close(null);
    }
}
