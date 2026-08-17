import { Injectable, inject } from '@angular/core';
import { I18nService } from '../shared/i18n/i18n.service';
import { UiToastService } from '../ui/service/ui-toast.service';

/** Resolves an i18n key and enqueues a toast via {@link UiToastService}. */
@Injectable({ providedIn: 'root' })
export class ToastNotificationService {
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(I18nService);

    public showError(translateKey: string, life = 4000): void {
        this.i18n.get$(translateKey).subscribe(detail => {
            this.toast.show({ severity: 'error', detail, life });
        });
    }

    public showSuccess(translateKey: string, life = 3000): void {
        this.i18n.get$(translateKey).subscribe(detail => {
            this.toast.show({ severity: 'success', detail, life });
        });
    }

    public showInfo(translateKey: string, life = 3000): void {
        this.i18n.get$(translateKey).subscribe(detail => {
            this.toast.show({ severity: 'info', detail, life });
        });
    }
}
