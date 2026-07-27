import { Injector, runInInjectionContext } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { of } from 'rxjs';
import { UiToastService } from '../ui/service/ui-toast.service';
import { ToastNotificationService } from './toast-notification.service';

function make() {
    const show = vi.fn();
    const translate = { get: (k: string) => of(`t:${k}`) } as unknown as TranslateService;
    const injector = Injector.create({
        providers: [
            { provide: UiToastService, useValue: { show } },
            { provide: TranslateService, useValue: translate }
        ]
    });
    const service = runInInjectionContext(injector, () => new ToastNotificationService());
    return { service, show };
}

describe('ToastNotificationService', () => {
    it('showError resolves the key and enqueues an error toast with default life 4000', () => {
        const { service, show } = make();
        service.showError('K.ERR');
        expect(show).toHaveBeenCalledWith({ severity: 'error', detail: 't:K.ERR', life: 4000 });
    });

    it('showSuccess enqueues a success toast with default life 3000', () => {
        const { service, show } = make();
        service.showSuccess('K.OK');
        expect(show).toHaveBeenCalledWith({ severity: 'success', detail: 't:K.OK', life: 3000 });
    });

    it('showInfo enqueues an info toast with default life 3000', () => {
        const { service, show } = make();
        service.showInfo('K.INFO');
        expect(show).toHaveBeenCalledWith({ severity: 'info', detail: 't:K.INFO', life: 3000 });
    });

    it('passes a custom life through', () => {
        const { service, show } = make();
        service.showError('K.ERR', 9000);
        expect(show).toHaveBeenCalledWith({ severity: 'error', detail: 't:K.ERR', life: 9000 });
    });
});
