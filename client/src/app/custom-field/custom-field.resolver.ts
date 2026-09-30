import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { CustomFieldStore } from './store/custom-field.store';

@Injectable({ providedIn: 'root' })
export class CustomFieldResolver implements Resolve<void> {
    private customFieldStore = inject(CustomFieldStore);

    public resolve(): void {
        this.customFieldStore.load();
    }
}
