import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { TagStore } from './store/tag.store';

@Injectable({ providedIn: 'root' })
export class TagResolver implements Resolve<void> {
    private readonly tagStore = inject(TagStore);

    public resolve(): void {
        this.tagStore.load();
    }
}
