import { Injectable, inject } from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { BULK_SELECTION_LIMIT, SelectionResult } from '../store/issue-selection.store';

@Injectable({ providedIn: 'root' })
export class SelectionLimitToastService {
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(I18nService);

    public notify(result: SelectionResult): void {
        if (result !== 'limit') return;
        this.toast.show({
            severity: 'info',
            detail: this.i18n.instant('ISSUE.BULK_EDIT.LIMIT', { limit: BULK_SELECTION_LIMIT }),
            life: 4000
        });
    }
}
